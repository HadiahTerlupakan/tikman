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

	commands := []string{"configure terminal"}
	commands = append(commands, buildZTEInterfaceSection(req, onuID)...)
	commands = append(commands, buildZTEManagementSection(req, onuID, includePassword)...)

	return append(commands, "commit"), nil
}

// buildZTEInterfaceSection writes the transport: the bandwidth a T-CONT gets,
// the GEM port carrying it, and how the UNI's traffic is tagged.
func buildZTEInterfaceSection(req models.ZTEGPONRegisterRequest, onuID int) []string {
	commands := []string{fmt.Sprintf("interface gpon-onu_1/%d/%d:%d", req.Card, req.PON, onuID)}
	if name := strings.TrimSpace(req.Name); name != "" {
		commands = append(commands, fmt.Sprintf("name %s", name))
	}

	// "profile", not "profile-name": the latter is rejected as invalid input on
	// a C300 V2.1.0, whose own running config reads "tcont 1 name X profile Y".
	commands = append(commands,
		fmt.Sprintf("tcont 1 name %s profile %s", zteServiceLabel(req), req.DownloadProfile),
		fmt.Sprintf("gemport 1 name %s tcont 1", zteServiceLabel(req)),
		buildZTEServicePort(req),
		"exit",
	)

	return commands
}

// buildZTEServicePort maps the VLAN mode onto the service-port line. Untagged
// UNI traffic still leaves the OLT on the service VLAN, so only the user side
// changes.
func buildZTEServicePort(req models.ZTEGPONRegisterRequest) string {
	if req.VLANMode == models.ZTEVLANModeUntag {
		return fmt.Sprintf("service-port 1 vport 1 user-vlan untagged vlan %d", req.VLANID)
	}
	return fmt.Sprintf("service-port 1 vport 1 user-vlan %d vlan %d", req.VLANID, req.VLANID)
}

// buildZTEManagementSection writes the OMCI service and WAN. A bridged ONU, or
// one whose WAN is set up on the ONT itself, gets neither: the OLT only has to
// carry its traffic.
func buildZTEManagementSection(req models.ZTEGPONRegisterRequest, onuID int, includePassword bool) []string {
	if req.ServiceType == models.ZTEServiceBridge || req.WANMode != models.ZTEWANModeWANIP {
		return nil
	}

	commands := []string{
		fmt.Sprintf("pon-onu-mng gpon-onu_1/%d/%d:%d", req.Card, req.PON, onuID),
		buildZTEONUService(req),
	}

	switch req.WANIPMode {
	case models.ZTEWANIPModePPPoE:
		password := "<redacted>"
		if includePassword {
			password = req.PPPoEPassword
		}
		commands = append(commands, fmt.Sprintf(
			"wan-ip 1 mode pppoe username %s password %s vlan-profile %s host 1",
			req.PPPoEUsername, password, req.VLANProfile))
	default:
		commands = append(commands, fmt.Sprintf("wan-ip 1 mode %s vlan-profile %s host 1", req.WANIPMode, req.VLANProfile))
	}

	return append(commands,
		"wan 1 service internet host 1",
		"exit",
	)
}

func buildZTEONUService(req models.ZTEGPONRegisterRequest) string {
	if req.VLANMode == models.ZTEVLANModeUntag {
		return fmt.Sprintf("service %s gemport 1 untag", zteServiceLabel(req))
	}
	return fmt.Sprintf("service %s gemport 1 vlan %d", zteServiceLabel(req), req.VLANID)
}

func zteServiceLabel(req models.ZTEGPONRegisterRequest) string {
	if req.ServiceType == models.ZTEServiceBridge {
		return "bridge"
	}
	return "internet"
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
	if (strings.TrimSpace(req.Name) != "" && !isZTEName(req.Name)) || !isZTECommandToken(req.ONUType) || !isZTECommandToken(req.DownloadProfile) || !isZTECommandToken(req.UploadProfile) {
		return fmt.Errorf("command fields contain unsupported characters")
	}
	if req.DownloadProfile != req.UploadProfile {
		return fmt.Errorf("download and upload profiles must match")
	}
	// Re-checked here, not only in the request validator: this is the last point
	// before the commands reach the OLT, and an unrecognised mode must not fall
	// through to a default that provisions something else.
	if req.VLANMode != models.ZTEVLANModeTag && req.VLANMode != models.ZTEVLANModeUntag {
		return fmt.Errorf("unsupported VLAN mode %q", req.VLANMode)
	}
	if req.ServiceType != models.ZTEServiceInternet && req.ServiceType != models.ZTEServiceBridge {
		return fmt.Errorf("unsupported service type %q", req.ServiceType)
	}
	if req.WANMode != models.ZTEWANModeWANIP && req.WANMode != models.ZTEWANModeSetupViaONT {
		return fmt.Errorf("unsupported WAN mode %q", req.WANMode)
	}
	if req.WANMode == models.ZTEWANModeWANIP {
		switch req.WANIPMode {
		case models.ZTEWANIPModePPPoE, models.ZTEWANIPModeDHCP, models.ZTEWANIPModeStatic:
		default:
			return fmt.Errorf("unsupported WAN-IP mode %q", req.WANIPMode)
		}
		if !isZTECommandToken(req.VLANProfile) {
			return fmt.Errorf("command fields contain unsupported characters")
		}
		// The credentials are interpolated straight into a CLI line, so they are
		// re-checked here even though the request validator already did: this is
		// the boundary where a metacharacter would reach the OLT.
		if req.WANIPMode == models.ZTEWANIPModePPPoE &&
			(!isZTECredential(req.PPPoEUsername) || !isZTECredential(req.PPPoEPassword)) {
			return fmt.Errorf("command fields contain unsupported characters")
		}
	}
	if !req.ServiceEnabled {
		return fmt.Errorf("service must be enabled")
	}
	if req.VLANID < 1 || req.VLANID > 4094 {
		return fmt.Errorf("VLAN ID must be in range 1-4094")
	}
	return nil
}
