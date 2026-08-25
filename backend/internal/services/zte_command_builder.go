package services

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tikman/olt-provisioning/internal/models"
)

var ztePasswordPattern = regexp.MustCompile(`(?i)(\bpassword\s+)(\S+)`)

// BuildZTEGPONRegisterCommands creates redacted ordered CLI commands for an
// operator preview. Use BuildZTEGPONRegisterExecutionCommands only at the
// execution boundary where the password is sent to the OLT.
func BuildZTEGPONRegisterCommands(req models.ZTEGPONRegisterRequest, onuID int) ([]string, error) {
	return buildZTEGPONRegisterCommands(req, onuID, false)
}

// BuildZTEGPONRegisterExecutionCommands creates the runtime commands sent to
// the OLT. Callers must not log, persist, or return its result.
func BuildZTEGPONRegisterExecutionCommands(req models.ZTEGPONRegisterRequest, onuID int) ([]string, error) {
	return buildZTEGPONRegisterCommands(req, onuID, true)
}

// BuildZTEGPONServiceCommands creates redacted commands for an already
// registered ONU. The OLT must already have the ONU in its gpon-onu context.
func BuildZTEGPONServiceCommands(req models.ZTEGPONRegisterRequest, onuID int) ([]string, error) {
	return buildZTEGPONServiceCommands(req, onuID, false)
}

// BuildZTEGPONServiceExecutionCommands creates runtime service commands for an
// already registered ONU. Callers must not log, persist, or return its result.
func BuildZTEGPONServiceExecutionCommands(req models.ZTEGPONRegisterRequest, onuID int) ([]string, error) {
	return buildZTEGPONServiceCommands(req, onuID, true)
}

func buildZTEGPONRegisterCommands(req models.ZTEGPONRegisterRequest, onuID int, includePassword bool) ([]string, error) {
	if err := validateZTECommandRequest(req, onuID); err != nil {
		return nil, err
	}

	service, err := buildZTEGPONServiceCommands(req, onuID, includePassword)
	if err != nil {
		return nil, err
	}
	serial := strings.ToUpper(strings.TrimSpace(req.SerialNumber))
	registration := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-olt_1/%d/%d", req.Card, req.PON),
		fmt.Sprintf("onu %d type %s sn %s", onuID, req.ONUType, serial),
		"exit",
	}
	return append(registration, service[1:]...), nil
}

func buildZTEGPONServiceCommands(req models.ZTEGPONRegisterRequest, onuID int, includePassword bool) ([]string, error) {
	if err := validateZTECommandRequest(req, onuID); err != nil {
		return nil, err
	}

	commands := []string{
		"configure terminal",
		fmt.Sprintf("interface gpon-onu_1/%d/%d:%d", req.Card, req.PON, onuID),
	}
	if name := strings.TrimSpace(req.Name); name != "" {
		commands = append(commands, fmt.Sprintf("name %s", name))
	}
	password := "<redacted>"
	if includePassword {
		password = req.PPPoEPassword
	}
	return append(commands,
		fmt.Sprintf("tcont 1 name internet profile-name %s", req.DownloadProfile),
		"gemport 1 name internet tcont 1",
		fmt.Sprintf("service-port 1 vport 1 user-vlan %d vlan %d", req.VLANID, req.VLANID),
		fmt.Sprintf("wan-ip 1 mode pppoe username %s password %s vlan-profile %s", req.PPPoEUsername, password, req.VLANProfile),
		"exit",
		"commit",
	), nil
}

// RedactZTECommands removes passwords from commands before they are logged or
// returned in an operator-facing preview.
func RedactZTECommands(commands []string) []string {
	redacted := make([]string, len(commands))
	for i, command := range commands {
		redacted[i] = ztePasswordPattern.ReplaceAllString(command, `${1}<redacted>`)
	}
	return redacted
}

func validateZTECommandRequest(req models.ZTEGPONRegisterRequest, onuID int) error {
	if req.Card <= 0 || req.PON <= 0 {
		return fmt.Errorf("card and PON must be positive")
	}
	if onuID < minZTEONUID || onuID > maxZTEONUID {
		return fmt.Errorf("ONU ID must be in range 1-127")
	}
	if !zteSerialPattern.MatchString(strings.ToUpper(strings.TrimSpace(req.SerialNumber))) {
		return fmt.Errorf("serial number must be 12 uppercase alphanumeric characters")
	}
	if strings.TrimSpace(req.ONUType) == "" {
		return fmt.Errorf("ONU type is required")
	}
	if (strings.TrimSpace(req.Name) != "" && !isZTEName(req.Name)) || !isZTECommandToken(req.ONUType) || !isZTECommandToken(req.DownloadProfile) || !isZTECommandToken(req.UploadProfile) || !isZTECommandToken(req.VLANProfile) || !isZTECredential(req.PPPoEUsername) || !isZTECredential(req.PPPoEPassword) {
		return fmt.Errorf("command fields contain unsupported characters")
	}
	if req.VLANMode != "tag" || req.DownloadProfile != req.UploadProfile {
		return fmt.Errorf("unsupported VLAN mode or profile combination")
	}
	if req.ServiceEnabled != true {
		return fmt.Errorf("service must be enabled")
	}
	if req.ServiceType != "internet" {
		return fmt.Errorf("service type must be internet")
	}
	if req.WANMode != "pppoe" {
		return fmt.Errorf("WAN mode must be pppoe")
	}
	if req.VLANID < 1 || req.VLANID > 4094 {
		return fmt.Errorf("VLAN ID must be in range 1-4094")
	}
	return nil
}
