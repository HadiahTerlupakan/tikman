package services

import (
	"context"
	"crypto/subtle"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/tikman/olt-provisioning/internal/librenms"
	"github.com/tikman/olt-provisioning/internal/models"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// MinWebhookTokenLength keeps the webhook's secret out of guessing range; a
// shorter one is treated as unset and refuses every call.
const MinWebhookTokenLength = 16

// portAlertRunTimeout covers one device's port read from LibreNMS, its
// retries, and the notices it leads to. Four reads at the client's 30 s
// timeout plus the waits between them leave over a minute for the notices.
const portAlertRunTimeout = 4 * time.Minute

// portReadRetryDelays are the waits before each new attempt at a device's
// ports. LibreNMS sends an alert once per state change and never again, so a
// read lost to a LibreNMS hiccup, or to the same tunnel flap that took the
// port down, would lose the outage and then its recovery: the recovery reads
// "up" against a stored "up". A variable so tests need not wait.
var portReadRetryDelays = []time.Duration{5 * time.Second, 15 * time.Second, 30 * time.Second}

// The ifOperStatus values LibreNMS reports for a port whose link is down.
// lowerLayerDown is what a LAG or a VLAN interface shows when what it rides
// on failed.
const (
	ifStatusUp             = "up"
	ifStatusDown           = "down"
	ifStatusLowerLayerDown = "lowerLayerDown"
)

// PortAlertService turns LibreNMS's alerts into port down/up notices. An
// alert only says that a device has something to report, and its API
// transport cannot send which ports; so the device's ports are read back from
// LibreNMS, which has already polled them when the alert fires, and compared
// with the last status recorded for each port a graph report names.
type PortAlertService struct {
	db       *gorm.DB
	sender   *GraphReportSender
	settings *SettingService
	logger   *zap.Logger
	now      func() time.Time

	mu sync.Mutex
	// running holds the devices being processed, and again those alerted
	// while they were: one more run follows, instead of two at once racing
	// to read the same ports.
	running map[int]bool
	again   map[int]bool
}

// NewPortAlertService constructs a PortAlertService that sends through the
// graph reports' sender: the same LibreNMS, WhatsApp queue and Telegram bot.
func NewPortAlertService(db *gorm.DB, sender *GraphReportSender, logger *zap.Logger) *PortAlertService {
	return &PortAlertService{
		db: db, sender: sender, settings: sender.settings, logger: logger, now: time.Now,
		running: map[int]bool{}, again: map[int]bool{},
	}
}

// WebhookAuthorized says whether token is the configured webhook secret.
// With none configured, or one too short to trust, nothing is.
func (s *PortAlertService) WebhookAuthorized(token string) bool {
	secret, err := s.settings.Value(models.SettingLibreNMSWebhookToken)
	if err != nil || len(secret) < MinWebhookTokenLength {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(token), []byte(secret)) == 1
}

// Trigger processes a device in the background. The webhook answers at once:
// LibreNMS waits on it, and a slow LibreNMS read would hold up its alerting.
func (s *PortAlertService) Trigger(deviceID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.running[deviceID] {
		s.again[deviceID] = true
		return
	}
	s.running[deviceID] = true
	go s.run(deviceID)
}

func (s *PortAlertService) run(deviceID int) {
	for {
		ctx, cancel := context.WithTimeout(context.Background(), portAlertRunTimeout)
		if err := s.Process(ctx, deviceID); err != nil {
			s.logger.Warn("port alert processing failed", zap.Int("device_id", deviceID), zap.Error(err))
		}
		cancel()
		s.mu.Lock()
		if !s.again[deviceID] {
			delete(s.running, deviceID)
			s.mu.Unlock()
			return
		}
		delete(s.again, deviceID)
		s.mu.Unlock()
	}
}

// Process reads a device's monitored ports, records each one's status, and
// sends a notice for every port whose status changed.
func (s *PortAlertService) Process(ctx context.Context, deviceID int) error {
	names, err := s.monitoredPorts(deviceID)
	if err != nil || len(names) == 0 {
		return err
	}
	ports, err := s.readPorts(ctx, deviceID)
	if err != nil {
		return err
	}
	byName := make(map[string]librenms.Port, len(ports))
	for _, port := range ports {
		byName[port.IfName] = port
	}
	now := s.now()
	var changed []portTransition
	var failures []error
	for _, name := range names {
		status := linkStatus(byName[name])
		if status == "" {
			continue
		}
		transition, err := s.record(deviceID, name, status, now)
		if err != nil {
			failures = append(failures, err)
			continue
		}
		if transition != nil {
			changed = append(changed, *transition)
		}
	}
	if len(changed) > 0 {
		failures = append(failures, s.notify(ctx, deviceID, changed))
	}
	return errors.Join(failures...)
}

// readPorts reads a device's ports from LibreNMS, trying again after each of
// portReadRetryDelays while the read fails. A LibreNMS left unconfigured is
// not retried: waiting will not configure it.
func (s *PortAlertService) readPorts(ctx context.Context, deviceID int) ([]librenms.Port, error) {
	for attempt := 0; ; attempt++ {
		client, err := s.sender.LibreNMS()
		if errors.Is(err, ErrLibreNMSNotConfigured) {
			return nil, err
		}
		var ports []librenms.Port
		if err == nil {
			ports, err = client.Ports(ctx, deviceID)
		}
		if err == nil {
			return ports, nil
		}
		if attempt == len(portReadRetryDelays) {
			return nil, fmt.Errorf("read ports of device %d: %w", deviceID, err)
		}
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("read ports of device %d: %w", deviceID, err)
		case <-time.After(portReadRetryDelays[attempt]):
		}
	}
}

// monitoredPorts answers the ifNames of a device that any graph report names.
func (s *PortAlertService) monitoredPorts(deviceID int) ([]string, error) {
	var names []string
	err := s.db.Model(&models.GraphReportPort{}).Where("device_id = ?", deviceID).
		Distinct().Order("if_name ASC").Pluck("if_name", &names).Error
	if err != nil {
		return nil, fmt.Errorf("list monitored ports: %w", err)
	}
	return names, nil
}

// linkStatus answers what a port's link is, or "" when it says nothing
// worth recording: a port an admin shut down is not an outage, and a port
// LibreNMS no longer lists, or reports as unknown or testing, is left as it
// was last seen.
func linkStatus(port librenms.Port) string {
	switch {
	case port.IfOperStatus == ifStatusUp:
		return models.PortLinkUp
	case port.IfAdminStatus != ifStatusUp:
		return ""
	case port.IfOperStatus == ifStatusDown, port.IfOperStatus == ifStatusLowerLayerDown:
		return models.PortLinkDown
	}
	return ""
}
