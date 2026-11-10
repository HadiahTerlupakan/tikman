package services

import (
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// MaxGraphReportPorts keeps one morning's report to a handful of images; past
// this a group reads it as spam.
const MaxGraphReportPorts = 10

// graphReportTextRunes matches the varchar(255) the names are stored in.
const graphReportTextRunes = 255

// dateLayout is how last_sent_on travels to the browser.
const dateLayout = "2006-01-02"

// ErrGraphReportDuplicate is a second report for a group that has one.
var ErrGraphReportDuplicate = errors.New("grup ini sudah memiliki laporan grafik")

// GraphReportPortInput is one port as the admin picked it.
type GraphReportPortInput struct {
	DeviceID   int
	DeviceName string
	IfName     string
	Label      string
}

// GraphReportInput is a report as the admin submitted it. Ports are sent in
// the order given.
type GraphReportInput struct {
	WAGroupID    uuid.UUID
	CustomerName string
	Enabled      bool
	Ports        []GraphReportPortInput
}

// GraphReportView is a report as the management page shows it.
type GraphReportView struct {
	ID            uuid.UUID                `json:"id"`
	WAGroupID     uuid.UUID                `json:"wa_group_id"`
	GroupName     string                   `json:"group_name"`
	CustomerName  string                   `json:"customer_name"`
	Enabled       bool                     `json:"enabled"`
	LastSentOn    *string                  `json:"last_sent_on"`
	LastAttemptAt *time.Time               `json:"last_attempt_at"`
	LastError     string                   `json:"last_error"`
	Ports         []models.GraphReportPort `json:"ports"`
}

// GraphReportService stores the daily graph reports.
type GraphReportService struct {
	db *gorm.DB
}

// NewGraphReportService constructs a GraphReportService.
func NewGraphReportService(db *gorm.DB) *GraphReportService {
	return &GraphReportService{db: db}
}

// List answers every report, by customer.
func (s *GraphReportService) List() ([]GraphReportView, error) {
	var reports []models.GraphReport
	if err := s.db.Order("customer_name ASC").Find(&reports).Error; err != nil {
		return nil, fmt.Errorf("list graph reports: %w", err)
	}
	return s.views(reports)
}

// Get answers one report.
func (s *GraphReportService) Get(id uuid.UUID) (*GraphReportView, error) {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return nil, err
	}
	views, err := s.views([]models.GraphReport{report})
	if err != nil {
		return nil, err
	}
	return &views[0], nil
}

// Create registers a report for a group that has none.
func (s *GraphReportService) Create(in GraphReportInput, actor uuid.UUID) (*GraphReportView, error) {
	if err := s.validate(in, uuid.Nil); err != nil {
		return nil, err
	}
	report := models.GraphReport{
		WAGroupID: in.WAGroupID, CustomerName: strings.TrimSpace(in.CustomerName),
		Enabled: in.Enabled, CreatedBy: &actor,
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		// Enabled is written explicitly: GORM skips a false bool on create
		// and the column default would turn the report on.
		if err := tx.Create(&report).Error; err != nil {
			return fmt.Errorf("create graph report: %w", err)
		}
		if err := tx.Model(&report).Update("enabled", in.Enabled).Error; err != nil {
			return fmt.Errorf("create graph report: %w", err)
		}
		return replacePorts(tx, report.ID, in.Ports)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(report.ID)
}

// Update rewrites a report and replaces its ports.
func (s *GraphReportService) Update(id uuid.UUID, in GraphReportInput) (*GraphReportView, error) {
	var report models.GraphReport
	if err := s.db.First(&report, "id = ?", id).Error; err != nil {
		return nil, err
	}
	if err := s.validate(in, id); err != nil {
		return nil, err
	}
	err := s.db.Transaction(func(tx *gorm.DB) error {
		err := tx.Model(&report).Updates(map[string]any{
			"wa_group_id": in.WAGroupID, "customer_name": strings.TrimSpace(in.CustomerName), "enabled": in.Enabled,
		}).Error
		if err != nil {
			return fmt.Errorf("update graph report: %w", err)
		}
		return replacePorts(tx, id, in.Ports)
	})
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}

// Delete removes a report with its ports. The ports are deleted here as well
// as by the foreign key because SQLite does not enforce it.
func (s *GraphReportService) Delete(id uuid.UUID) error {
	return s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("report_id = ?", id).Delete(&models.GraphReportPort{}).Error; err != nil {
			return fmt.Errorf("delete graph report ports: %w", err)
		}
		result := tx.Delete(&models.GraphReport{}, "id = ?", id)
		if result.Error != nil {
			return fmt.Errorf("delete graph report: %w", result.Error)
		}
		if result.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return nil
	})
}

func replacePorts(tx *gorm.DB, reportID uuid.UUID, ports []GraphReportPortInput) error {
	if err := tx.Where("report_id = ?", reportID).Delete(&models.GraphReportPort{}).Error; err != nil {
		return fmt.Errorf("clear graph report ports: %w", err)
	}
	rows := make([]models.GraphReportPort, len(ports))
	for i, port := range ports {
		rows[i] = models.GraphReportPort{
			ReportID: reportID, DeviceID: port.DeviceID, DeviceName: strings.TrimSpace(port.DeviceName),
			IfName: strings.TrimSpace(port.IfName), Label: strings.TrimSpace(port.Label), Position: i,
		}
	}
	if err := tx.Create(&rows).Error; err != nil {
		return fmt.Errorf("store graph report ports: %w", err)
	}
	return nil
}

func (s *GraphReportService) validate(in GraphReportInput, self uuid.UUID) error {
	if err := validateGraphReportFields(in); err != nil {
		return err
	}
	var group models.WAGroup
	err := s.db.Select("id").First(&group, "id = ?", in.WAGroupID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("%w: grup WhatsApp tidak ditemukan", ErrValidation)
	}
	if err != nil {
		return fmt.Errorf("look up group: %w", err)
	}
	var taken int64
	err = s.db.Model(&models.GraphReport{}).Where("wa_group_id = ? AND id <> ?", in.WAGroupID, self).Count(&taken).Error
	if err != nil {
		return fmt.Errorf("look for existing graph report: %w", err)
	}
	if taken > 0 {
		return ErrGraphReportDuplicate
	}
	return nil
}

func validateGraphReportFields(in GraphReportInput) error {
	if err := requireText(in.CustomerName, "nama pelanggan"); err != nil {
		return err
	}
	if len(in.Ports) == 0 || len(in.Ports) > MaxGraphReportPorts {
		return fmt.Errorf("%w: jumlah port harus 1 sampai %d", ErrValidation, MaxGraphReportPorts)
	}
	for i, port := range in.Ports {
		if port.DeviceID <= 0 {
			return fmt.Errorf("%w: port ke-%d belum memilih perangkat", ErrValidation, i+1)
		}
		if err := validateGraphReportPort(port, i+1); err != nil {
			return err
		}
	}
	return nil
}

func validateGraphReportPort(port GraphReportPortInput, n int) error {
	if utf8.RuneCountInString(strings.TrimSpace(port.DeviceName)) > graphReportTextRunes {
		return fmt.Errorf("%w: nama perangkat port ke-%d paling panjang %d karakter", ErrValidation, n, graphReportTextRunes)
	}
	if err := requireText(port.IfName, fmt.Sprintf("nama interface port ke-%d", n)); err != nil {
		return err
	}
	return requireText(port.Label, fmt.Sprintf("label port ke-%d", n))
}

func requireText(value, name string) error {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fmt.Errorf("%w: %s wajib diisi", ErrValidation, name)
	}
	if utf8.RuneCountInString(trimmed) > graphReportTextRunes {
		return fmt.Errorf("%w: %s paling panjang %d karakter", ErrValidation, name, graphReportTextRunes)
	}
	return nil
}

// views attaches the group names and ports in two queries for the whole page.
func (s *GraphReportService) views(reports []models.GraphReport) ([]GraphReportView, error) {
	ids := make([]uuid.UUID, len(reports))
	groupIDs := make([]uuid.UUID, len(reports))
	for i, report := range reports {
		ids[i], groupIDs[i] = report.ID, report.WAGroupID
	}
	var ports []models.GraphReportPort
	if err := s.db.Where("report_id IN ?", ids).Order("position ASC").Find(&ports).Error; err != nil {
		return nil, fmt.Errorf("load graph report ports: %w", err)
	}
	var groups []models.WAGroup
	if err := s.db.Select("id", "name").Where("id IN ?", groupIDs).Find(&groups).Error; err != nil {
		return nil, fmt.Errorf("load graph report groups: %w", err)
	}
	names := make(map[uuid.UUID]string, len(groups))
	for _, group := range groups {
		names[group.ID] = group.Name
	}
	byReport := make(map[uuid.UUID][]models.GraphReportPort, len(reports))
	for _, port := range ports {
		byReport[port.ReportID] = append(byReport[port.ReportID], port)
	}
	views := make([]GraphReportView, len(reports))
	for i, report := range reports {
		views[i] = graphReportView(report, names[report.WAGroupID], byReport[report.ID])
	}
	return views, nil
}

func graphReportView(report models.GraphReport, groupName string, ports []models.GraphReportPort) GraphReportView {
	view := GraphReportView{
		ID: report.ID, WAGroupID: report.WAGroupID, GroupName: groupName, CustomerName: report.CustomerName,
		Enabled: report.Enabled, LastAttemptAt: report.LastAttemptAt, LastError: report.LastError, Ports: ports,
	}
	if view.Ports == nil {
		view.Ports = []models.GraphReportPort{}
	}
	if report.LastSentOn != nil {
		day := report.LastSentOn.UTC().Format(dateLayout)
		view.LastSentOn = &day
	}
	return view
}
