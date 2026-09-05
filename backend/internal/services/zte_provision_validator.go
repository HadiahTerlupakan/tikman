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
// What the C300 CLI itself accepts in these fields.
const (
	maxZTEONUTypeLength = 64
	minVLANID           = 1
	maxVLANID           = 4094
)

func ValidateZTEGPONRegister(req models.ZTEGPONRegisterRequest, olt *models.OLT) error {
	if olt == nil || (olt.Model != models.OLTModelZTEC300 && olt.Model != models.OLTModelZTEC320) {
		return fmt.Errorf("ZTE GPON registration supports only C300 or C320 OLTs")
	}
	if err := validateZTEPosition(req); err != nil {
		return err
	}
	if err := validateZTEIdentity(req); err != nil {
		return err
	}
	if err := validateZTEService(req); err != nil {
		return err
	}
	return validateZTEWAN(req)
}

// validateZTEPosition checks where on the chassis the ONU is being put.
func validateZTEPosition(req models.ZTEGPONRegisterRequest) error {
	if req.Card < 1 || req.Card > connectivity.MaxBoardID {
		return fmt.Errorf("card must be in range 1-%d", connectivity.MaxBoardID)
	}
	if req.PON < 1 || req.PON > connectivity.MaxPonID {
		return fmt.Errorf("PON must be in range 1-%d", connectivity.MaxPonID)
	}

	switch req.ONUIDMode {
	case models.ZTEONUIDAuto:
		if req.ONUID != 0 {
			return fmt.Errorf("auto ONU ID must be zero")
		}
	case models.ZTEONUIDCustom:
		if req.ONUID < minZTEONUID || req.ONUID > maxZTEONUID {
			return fmt.Errorf("custom ONU ID must be in range %d-%d", minZTEONUID, maxZTEONUID)
		}
	default:
		return fmt.Errorf("ONU ID mode must be auto or custom")
	}
	return nil
}

// validateZTEIdentity checks the values that end up inside a CLI command, so
// anything the chassis would read as a second command is refused here.
func validateZTEIdentity(req models.ZTEGPONRegisterRequest) error {
	if !zteSerialPattern.MatchString(strings.ToUpper(strings.TrimSpace(req.SerialNumber))) {
		return fmt.Errorf("serial number must be 12 uppercase alphanumeric characters")
	}
	if strings.TrimSpace(req.ONUType) == "" {
		return fmt.Errorf("ONU type is required")
	}
	if len([]rune(req.ONUType)) > maxZTEONUTypeLength {
		return fmt.Errorf("ONU type must be at most %d characters", maxZTEONUTypeLength)
	}
	if !isZTECommandToken(req.ONUType) {
		return fmt.Errorf("ONU type contains unsupported characters")
	}
	if strings.TrimSpace(req.Name) != "" && !isZTEName(req.Name) {
		return fmt.Errorf("ONU name contains unsupported characters")
	}
	if strings.TrimSpace(req.Description) != "" && !isZTEName(req.Description) {
		return fmt.Errorf("ONU description contains unsupported characters")
	}
	return nil
}

// validateZTEService checks what the subscriber will actually be given.
func validateZTEService(req models.ZTEGPONRegisterRequest) error {
	if req.VLANMode != models.ZTEVLANModeTag && req.VLANMode != models.ZTEVLANModeUntag {
		return fmt.Errorf("VLAN mode must be tag or untag")
	}
	if !isZTECommandToken(req.DownloadProfile) || !isZTECommandToken(req.UploadProfile) {
		return fmt.Errorf("profile contains unsupported characters")
	}
	if req.DownloadProfile != req.UploadProfile {
		return fmt.Errorf("download and upload profiles must match")
	}
	if !req.ServiceEnabled {
		return fmt.Errorf("service must be enabled")
	}
	if req.VLANID < minVLANID || req.VLANID > maxVLANID {
		return fmt.Errorf("VLAN ID must be in range %d-%d", minVLANID, maxVLANID)
	}
	if req.ServiceType != models.ZTEServiceInternet && req.ServiceType != models.ZTEServiceBridge {
		return fmt.Errorf("service type must be internet or bridge")
	}
	return nil
}

// validateZTEWAN checks the WAN half of the request. A bridged ONU and an ONU
// set up on the ONT itself both carry no OMCI WAN, so everything below the WAN
// mode has to be absent rather than merely ignored: silently dropping a filled
// field would provision something other than what the operator saw.
func validateZTEWAN(req models.ZTEGPONRegisterRequest) error {
	if req.WANMode != models.ZTEWANModeWANIP && req.WANMode != models.ZTEWANModeSetupViaONT {
		return fmt.Errorf("WAN mode must be wan_ip or setup_via_ont")
	}

	if req.ServiceType == models.ZTEServiceBridge && req.WANMode != models.ZTEWANModeSetupViaONT {
		return fmt.Errorf("a bridge service carries no OMCI WAN, so WAN mode must be setup_via_ont")
	}
	if req.VLANMode == models.ZTEVLANModeUntag && req.WANMode != models.ZTEWANModeSetupViaONT {
		return fmt.Errorf("an untagged service carries no OMCI WAN, so WAN mode must be setup_via_ont")
	}

	if req.WANMode == models.ZTEWANModeSetupViaONT {
		if req.WANIPMode != "" || req.VLANProfile != "" || req.PPPoEUsername != "" || req.PPPoEPassword != "" {
			return fmt.Errorf("WAN details do not apply when the WAN is set up on the ONT")
		}
		return nil
	}

	switch req.WANIPMode {
	case models.ZTEWANIPModePPPoE, models.ZTEWANIPModeDHCP, models.ZTEWANIPModeStatic:
	default:
		return fmt.Errorf("WAN-IP mode must be pppoe, dhcp or static")
	}

	if !isZTECommandToken(req.VLANProfile) {
		return fmt.Errorf("VLAN profile contains unsupported characters")
	}

	if req.WANIPMode != models.ZTEWANIPModePPPoE {
		if req.PPPoEUsername != "" || req.PPPoEPassword != "" {
			return fmt.Errorf("PPPoE credentials do not apply to a %s WAN", req.WANIPMode)
		}
		return nil
	}

	return validateZTEPPPoECredentials(req)
}

func validateZTEPPPoECredentials(req models.ZTEGPONRegisterRequest) error {
	if req.PPPoEUsername == "" {
		return fmt.Errorf("PPPoE username is required")
	}
	if !isZTECredential(req.PPPoEUsername) {
		return fmt.Errorf("PPPoE username contains unsupported characters")
	}
	if strings.IndexFunc(req.PPPoEUsername, unicodeWhitespace) >= 0 {
		return fmt.Errorf("PPPoE username cannot contain whitespace")
	}
	if req.PPPoEPassword == "" {
		return fmt.Errorf("PPPoE password is required")
	}
	if !isZTECredential(req.PPPoEPassword) {
		return fmt.Errorf("PPPoE password contains unsupported characters")
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
