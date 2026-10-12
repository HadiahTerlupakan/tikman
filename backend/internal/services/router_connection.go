package services

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/mikrotik"
	"github.com/tikman/olt-provisioning/internal/models"
	"github.com/tikman/olt-provisioning/internal/utils"
)

// routerSNMPTestTimeout matches the per-request timeout the poller uses, so a
// router that passes the test is not one the poller then times out on.
const routerSNMPTestTimeout = 2 * time.Second

// ErrFingerprintMismatch refuses a certificate the router was not seen
// presenting.
var ErrFingerprintMismatch = errors.New("fingerprint does not match the certificate the router presented")

// ConnectionTestInput is what the add-router form has before anything is
// stored.
type ConnectionTestInput struct {
	Host           string
	SNMPPort       int
	SNMPCommunity  string
	APIPort        int
	APIUsername    string
	APIPassword    string
	TLSFingerprint string
}

// ConnectionTestResult reports SNMP and REST separately: either can work while
// the other does not, and the operator fixes them in different places.
type ConnectionTestResult struct {
	SNMPError   string `json:"snmp_error,omitempty"`
	APIError    string `json:"api_error,omitempty"`
	Fingerprint string `json:"fingerprint,omitempty"`
	BoardName   string `json:"board_name,omitempty"`
	Version     string `json:"version,omitempty"`
}

// TestConnection runs in two steps. Without a fingerprint it only reads the
// certificate the router presents, so the operator can compare it with Winbox
// before any password is sent. With the confirmed fingerprint it pins REST to
// that certificate and reads the board. Splitting the steps keeps a
// man-in-the-middle at add time from receiving the API password.
func (s *RouterService) TestConnection(in ConnectionTestInput) ConnectionTestResult {
	var result ConnectionTestResult
	snmpPort := portOr(in.SNMPPort, DefaultRouterSNMPPort)
	if err := connectivity.SNMPTest(in.Host, snmpPort, in.SNMPCommunity, routerSNMPTestTimeout); err != nil {
		result.SNMPError = err.Error()
	}

	apiPort := portOr(in.APIPort, DefaultRouterAPIPort)
	if in.TLSFingerprint == "" {
		fingerprint, err := mikrotik.FetchFingerprint(in.Host, apiPort)
		if err != nil {
			result.APIError = err.Error()
		}
		result.Fingerprint = fingerprint
		return result
	}
	return s.readBoard(result, in, apiPort)
}

func (s *RouterService) readBoard(result ConnectionTestResult, in ConnectionTestInput, apiPort int) ConnectionTestResult {
	if !validFingerprint(in.TLSFingerprint) {
		result.APIError = "tls_fingerprint must be the 64-character SHA-256 the connection test returned"
		return result
	}
	fingerprint := strings.ToLower(in.TLSFingerprint)
	result.Fingerprint = fingerprint

	resource, err := mikrotik.NewClient(in.Host, apiPort, in.APIUsername, in.APIPassword, fingerprint).SystemResource()
	if err != nil {
		result.APIError = err.Error()
		return result
	}
	result.BoardName, result.Version = resource.BoardName, resource.Version
	return result
}

// Secrets decrypts a router's SNMP community and API password.
func (s *RouterService) Secrets(router *models.MikrotikRouter) (string, string, error) {
	community, err := utils.Decrypt(router.SNMPCommunity, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt community: %w", err)
	}
	password, err := utils.Decrypt(router.APIPassword, s.encryptionKey)
	if err != nil {
		return "", "", fmt.Errorf("decrypt password: %w", err)
	}
	return community, password, nil
}

// TrustCertificate pins the certificate the poller last saw the router
// presenting. The fingerprint must match it: an admin confirms what they were
// shown, not whatever the router happens to present by the time they click.
func (s *RouterService) TrustCertificate(id uuid.UUID, fingerprint string) (*models.MikrotikRouter, error) {
	router, err := s.Get(id)
	if err != nil {
		return nil, err
	}
	if router.PresentedFingerprint == nil || !strings.EqualFold(*router.PresentedFingerprint, fingerprint) {
		return nil, ErrFingerprintMismatch
	}
	err = s.db.Model(&models.MikrotikRouter{}).Where("id = ?", id).Updates(map[string]interface{}{
		"tls_fingerprint":       strings.ToLower(fingerprint),
		"presented_fingerprint": nil,
		"api_status":            models.RouterAPIStatusUnknown,
		"api_error":             nil,
	}).Error
	if err != nil {
		return nil, err
	}
	return s.Get(id)
}
