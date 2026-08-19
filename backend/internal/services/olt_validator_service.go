package services

import (
	"fmt"
	"time"

	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

// ValidationResult holds the result of OLT validation
type ValidationResult struct {
	Success      bool
	PassedTests  []string
	FailedTest   string
	FailedReason string
}

// OLTValidatorService handles OLT validation logic
type OLTValidatorService struct {
	db *gorm.DB
}

// NewOLTValidatorService creates a new validator service
func NewOLTValidatorService(db *gorm.DB) *OLTValidatorService {
	return &OLTValidatorService{
		db: db,
	}
}

// ValidateIPNotDuplicate checks if IP address already exists
func (s *OLTValidatorService) ValidateIPNotDuplicate(ipAddress string) error {
	var count int64
	if err := s.db.Model(&models.OLT{}).Where("ip_address = ?", ipAddress).Count(&count).Error; err != nil {
		return fmt.Errorf("failed to check IP duplicate: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("IP address already exists")
	}
	return nil
}

// ValidateCreate performs all connection tests in sequence
func (s *OLTValidatorService) ValidateCreate(ipAddress, username, password string, sshPort, telnetPort, snmpPort int, snmpCommunity string, preferredProtocol models.OLTProtocol) (*ValidationResult, error) {
	result := &ValidationResult{
		Success:     true,
		PassedTests: []string{},
	}

	// Test 1: Ping
	if err := connectivity.PingTest(ipAddress, 2*time.Second); err != nil {
		result.Success = false
		result.FailedTest = "Ping"
		result.FailedReason = err.Error()
		return result, nil
	}
	result.PassedTests = append(result.PassedTests, "Ping")

	// Test 2: SSH or Telnet based on preferred protocol
	if preferredProtocol == models.OLTProtocolSSH {
		if err := connectivity.SSHTest(ipAddress, sshPort, username, password, 2*time.Second); err != nil {
			result.Success = false
			result.FailedTest = "SSH"
			result.FailedReason = err.Error()
			return result, nil
		}
		result.PassedTests = append(result.PassedTests, "SSH")
	} else {
		if err := connectivity.TelnetTest(ipAddress, telnetPort, username, password, 2*time.Second); err != nil {
			result.Success = false
			result.FailedTest = "Telnet"
			result.FailedReason = err.Error()
			return result, nil
		}
		result.PassedTests = append(result.PassedTests, "Telnet")
	}

	// Test 3: SNMP (longer timeout for UDP)
	if err := connectivity.SNMPTest(ipAddress, snmpPort, snmpCommunity, 3*time.Second); err != nil {
		result.Success = false
		result.FailedTest = "SNMP"
		result.FailedReason = err.Error()
		return result, nil
	}
	result.PassedTests = append(result.PassedTests, "SNMP")

	return result, nil
}
