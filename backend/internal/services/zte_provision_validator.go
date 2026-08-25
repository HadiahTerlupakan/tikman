package services

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/google/uuid"
	"github.com/tikman/olt-provisioning/internal/connectivity"
	"github.com/tikman/olt-provisioning/internal/models"
	"gorm.io/gorm"
)

const (
	minZTEONUID = 1
	maxZTEONUID = 127
)

var zteSerialPattern = regexp.MustCompile(`^[A-Z0-9]{12}$`)

// ValidateZTEGPONRegister validates the supported ZTE GPON registration contract.
func ValidateZTEGPONRegister(req models.ZTEGPONRegisterRequest, olt *models.OLT) error {
	if olt == nil || (olt.Model != models.OLTModelZTEC300 && olt.Model != models.OLTModelZTEC320) {
		return fmt.Errorf("ZTE GPON registration supports only C300 or C320 OLTs")
	}
	if req.Card < 1 || req.Card > connectivity.MaxBoardID {
		return fmt.Errorf("card must be in range 1-%d", connectivity.MaxBoardID)
	}
	if req.PON < 1 || req.PON > connectivity.MaxPonID {
		return fmt.Errorf("PON must be in range 1-%d", connectivity.MaxPonID)
	}
	if !zteSerialPattern.MatchString(strings.ToUpper(strings.TrimSpace(req.SerialNumber))) {
		return fmt.Errorf("serial number must be 12 uppercase alphanumeric characters")
	}
	if strings.TrimSpace(req.ONUType) == "" {
		return fmt.Errorf("ONU type is required")
	}
	if len([]rune(req.ONUType)) > 64 {
		return fmt.Errorf("ONU type must be at most 64 characters")
	}
	if !isZTECommandToken(req.ONUType) {
		return fmt.Errorf("ONU type contains unsupported characters")
	}
	if strings.TrimSpace(req.Name) != "" && !isZTEName(req.Name) {
		return fmt.Errorf("ONU name contains unsupported characters")
	}
	if req.VLANMode != "tag" {
		return fmt.Errorf("VLAN mode must be tag")
	}
	if !isZTECommandToken(req.DownloadProfile) || !isZTECommandToken(req.UploadProfile) || !isZTECommandToken(req.VLANProfile) {
		return fmt.Errorf("profile contains unsupported characters")
	}
	if req.DownloadProfile != req.UploadProfile {
		return fmt.Errorf("download and upload profiles must match")
	}
	if !isZTECredential(req.PPPoEUsername) {
		return fmt.Errorf("PPPoE username contains unsupported characters")
	}
	if !isZTECredential(req.PPPoEPassword) {
		return fmt.Errorf("PPPoE password contains unsupported characters")
	}
	switch req.ONUIDMode {
	case models.ZTEONUIDAuto:
		if req.ONUID != 0 {
			return fmt.Errorf("auto ONU ID must be zero")
		}
	case models.ZTEONUIDCustom:
		if req.ONUID < minZTEONUID || req.ONUID > maxZTEONUID {
			return fmt.Errorf("custom ONU ID must be in range 1-127")
		}
	default:
		return fmt.Errorf("ONU ID mode must be auto or custom")
	}
	if !req.ServiceEnabled {
		return fmt.Errorf("service must be enabled")
	}
	if req.VLANID < 1 || req.VLANID > 4094 {
		return fmt.Errorf("VLAN ID must be in range 1-4094")
	}
	if req.WANMode != "pppoe" {
		return fmt.Errorf("WAN mode must be pppoe")
	}
	if req.ServiceType != "internet" {
		return fmt.Errorf("service type must be internet")
	}
	if req.PPPoEUsername == "" {
		return fmt.Errorf("PPPoE username is required")
	}
	if strings.IndexFunc(req.PPPoEUsername, unicodeWhitespace) >= 0 {
		return fmt.Errorf("PPPoE username cannot contain whitespace")
	}
	if req.PPPoEPassword == "" {
		return fmt.Errorf("PPPoE password is required")
	}
	if strings.IndexFunc(req.PPPoEPassword, unicodeWhitespace) >= 0 {
		return fmt.Errorf("PPPoE password cannot contain whitespace")
	}
	return nil
}

func unicodeWhitespace(r rune) bool {
	return unicode.IsSpace(r)
}

func isZTECommandToken(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			continue
		}
		return false
	}
	return true
}

func isZTEName(value string) bool {
	if strings.TrimSpace(value) != value || value == "" {
		return false
	}
	for _, r := range value {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' || r == '.' || r == ' ' {
			continue
		}
		return false
	}
	return true
}

func isZTECredential(value string) bool {
	if value == "" || strings.IndexFunc(value, unicodeWhitespace) >= 0 {
		return false
	}
	for _, r := range value {
		if unicode.IsControl(r) || strings.ContainsRune(";'|`$\\\"", r) {
			return false
		}
	}
	return true
}

// ResolveZTEONUID returns a free ONU ID for an OLT PON port or verifies a requested custom ID.
func ResolveZTEONUID(ctx context.Context, db *gorm.DB, oltID uuid.UUID, slotID, portID, requestedID int) (int, error) {
	if slotID < 1 || slotID > connectivity.MaxBoardID || portID < 1 || portID > connectivity.MaxPonID {
		return 0, fmt.Errorf("card and PON must be in valid range")
	}
	if requestedID < 0 || requestedID > maxZTEONUID {
		return 0, fmt.Errorf("ONU ID must be zero for auto mode or in range 1-127")
	}

	var usedIDs []int
	if err := db.WithContext(ctx).Model(&models.ONT{}).
		Where("olt_id = ? AND slot = ? AND port_id = ?", oltID, slotID, portID).
		Pluck("ont_id", &usedIDs).Error; err != nil {
		return 0, fmt.Errorf("failed to resolve ONU ID: %w", err)
	}
	used := make(map[int]struct{}, len(usedIDs))
	for _, id := range usedIDs {
		used[id] = struct{}{}
	}
	if requestedID != 0 {
		if _, exists := used[requestedID]; exists {
			return 0, fmt.Errorf("ONU ID %d is already used on this port", requestedID)
		}
		return requestedID, nil
	}
	for id := minZTEONUID; id <= maxZTEONUID; id++ {
		if _, exists := used[id]; !exists {
			return id, nil
		}
	}
	return 0, fmt.Errorf("no free ONU IDs remain on this port")
}
