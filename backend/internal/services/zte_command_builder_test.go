package services

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestBuildZTEGPONRegisterCommands(t *testing.T) {
	req := models.ZTEGPONRegisterRequest{
		OLTID:           uuid.New(),
		Card:            3,
		PON:             1,
		SerialNumber:    "HWTCB403E8A0",
		ONUType:         "HG8245H5",
		Name:            "258179206252-Saraswati",
		ServiceEnabled:  true,
		VLANMode:        "tag",
		ServiceType:     "internet",
		VLANID:          100,
		DownloadProfile: "100M",
		UploadProfile:   "100M",
		WANMode:         "wan_ip",
		WANIPMode:       "pppoe",
		VLANProfile:     "INTERNET",
		PPPoEUsername:   "example-user",
		PPPoEPassword:   "secret-password",
	}

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)
	require.NotContains(t, strings.Join(commands, "\n"), req.PPPoEPassword)
	// "profile", not "profile-name", and the WAN lines under pon-onu-mng rather
	// than the interface: both match what a C300 V2.1.0 accepts and what its own
	// running config reads back.
	require.Equal(t, []string{
		"configure terminal",
		"interface gpon-olt_1/3/1",
		"onu 7 type HG8245H5 sn HWTCB403E8A0",
		"exit",
		"interface gpon-onu_1/3/1:7",
		"name 258179206252-Saraswati",
		"tcont 1 name internet profile 100M",
		"gemport 1 name internet tcont 1",
		"service-port 1 vport 1 user-vlan 100 vlan 100",
		"exit",
		"pon-onu-mng gpon-onu_1/3/1:7",
		"service internet gemport 1 vlan 100",
		"wan-ip 1 mode pppoe username example-user password <redacted> vlan-profile INTERNET host 1",
		"wan 1 service internet host 1",
		"exit",
		"commit",
	}, commands)
}

// An untagged service tags nothing on the user side but still leaves the OLT on
// the service VLAN, and carries no OMCI WAN of its own.
func TestBuildZTEGPONRegisterCommandsForUntaggedService(t *testing.T) {
	req := validZTECommandRequest()
	req.VLANMode = models.ZTEVLANModeUntag
	req.WANMode = models.ZTEWANModeSetupViaONT
	req.WANIPMode = ""
	req.VLANProfile = ""
	req.PPPoEUsername = ""
	req.PPPoEPassword = ""

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)

	joined := strings.Join(commands, "\n")
	require.Contains(t, joined, "service-port 1 vport 1 user-vlan untagged vlan 100")
	// The service line stays: it creates the vport the service-port binds to.
	require.Contains(t, joined, "service internet gemport 1 untag")
	require.NotContains(t, joined, "wan-ip")
}

// A bridged ONU carries traffic and nothing else: no OMCI service, no WAN.
func TestBuildZTEGPONRegisterCommandsForBridgeService(t *testing.T) {
	req := validZTECommandRequest()
	req.ServiceType = models.ZTEServiceBridge
	req.WANMode = models.ZTEWANModeSetupViaONT
	req.WANIPMode = ""
	req.VLANProfile = ""
	req.PPPoEUsername = ""
	req.PPPoEPassword = ""

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)

	joined := strings.Join(commands, "\n")
	require.Contains(t, joined, "tcont 1 name bridge profile 100M")
	// Without this the OLT refused the service-port line with a bare failure,
	// leaving a registered ONU carrying no configuration at all.
	require.Contains(t, joined, "service bridge gemport 1 vlan 100")
	require.NotContains(t, joined, "wan 1 service")
	require.NotContains(t, joined, "wan-ip")
}

// A Fiberhome, VSOL or Huawei HGU only takes the service once its virtual
// Ethernet port is bound; the syntax is the OLT's own.
func TestBuildZTEGPONRegisterCommandsBindsTheVEIPPort(t *testing.T) {
	req := validZTECommandRequest()
	req.UseVEIP = true

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)

	require.Contains(t, commands, "vlan port veip_1 mode tag vlan 100")
}

func TestBuildZTEGPONRegisterCommandsOmitsTheVEIPPortWhenOff(t *testing.T) {
	commands, err := BuildZTEGPONRegisterCommands(validZTECommandRequest(), 7)
	require.NoError(t, err)

	require.NotContains(t, strings.Join(commands, "\n"), "veip")
}

// The binding describes the ONU's own port, so it applies to a bridged HGU
// too. It was refused there only because the management section used to be
// skipped for a bridge, which it no longer is.
func TestBuildZTEGPONRegisterCommandsBindsTheVEIPPortOnABridge(t *testing.T) {
	req := validZTECommandRequest()
	req.UseVEIP = true
	req.ServiceType = models.ZTEServiceBridge
	req.WANMode = models.ZTEWANModeSetupViaONT
	req.WANIPMode = ""
	req.VLANProfile = ""
	req.PPPoEUsername = ""
	req.PPPoEPassword = ""

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)

	joined := strings.Join(commands, "\n")
	require.Contains(t, joined, "vlan port veip_1 mode tag vlan 100")
	require.NotContains(t, joined, "wan-ip")
}

// A DHCP WAN needs no credentials, and none must appear on the line.
func TestBuildZTEGPONRegisterCommandsForDHCPWAN(t *testing.T) {
	req := validZTECommandRequest()
	req.WANIPMode = models.ZTEWANIPModeDHCP
	req.PPPoEUsername = ""
	req.PPPoEPassword = ""

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)

	joined := strings.Join(commands, "\n")
	require.Contains(t, joined, "wan-ip 1 mode dhcp vlan-profile INTERNET host 1")
	require.NotContains(t, joined, "username")
	require.NotContains(t, joined, "password")
}

func TestBuildZTEGPONRegisterExecutionCommandsIncludesPassword(t *testing.T) {
	req := validZTECommandRequest()
	commands, err := BuildZTEGPONRegisterExecutionCommands(req, 7)
	require.NoError(t, err)
	require.Contains(t, strings.Join(commands, "\\n"), req.PPPoEPassword)
}

func TestBuildZTEGPONRegisterCommandsRejectsCommandMetacharacters(t *testing.T) {
	req := validZTECommandRequest()
	req.ONUType = "HG8245H5;delete"

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.Error(t, err)
	require.Nil(t, commands)
}

func TestBuildZTEGPONRegisterCommandsRejectsMismatchedProfiles(t *testing.T) {
	req := validZTECommandRequest()
	req.UploadProfile = "50M"

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.Error(t, err)
	require.Nil(t, commands)
}

// The builder is the last point before the OLT, so an unrecognised mode must
// stop here rather than fall through to whatever the default branch writes.
func TestBuildZTEGPONRegisterCommandsRejectsInvalidServiceMode(t *testing.T) {
	for name, mutate := range map[string]func(*models.ZTEGPONRegisterRequest){
		"WAN mode":     func(r *models.ZTEGPONRegisterRequest) { r.WANMode = "ipoe" },
		"WAN-IP mode":  func(r *models.ZTEGPONRegisterRequest) { r.WANIPMode = "ipoe" },
		"VLAN mode":    func(r *models.ZTEGPONRegisterRequest) { r.VLANMode = "qinq" },
		"service type": func(r *models.ZTEGPONRegisterRequest) { r.ServiceType = "iptv" },
	} {
		t.Run(name, func(t *testing.T) {
			req := validZTECommandRequest()
			mutate(&req)

			commands, err := BuildZTEGPONRegisterCommands(req, 7)
			require.Error(t, err)
			require.Nil(t, commands)
		})
	}
}

func TestRedactZTECommands(t *testing.T) {
	commands := []string{
		"wan-ip 1 mode pppoe username example-user password secret-password vlan-profile INTERNET",
	}

	redacted := RedactZTECommands(commands)
	require.Equal(t, []string{
		"wan-ip 1 mode pppoe username example-user password <redacted> vlan-profile INTERNET",
	}, redacted)
	require.NotContains(t, redacted[0], "secret-password")
}

func validZTECommandRequest() models.ZTEGPONRegisterRequest {
	return models.ZTEGPONRegisterRequest{
		Card:            3,
		PON:             1,
		SerialNumber:    "HWTCB403E8A0",
		ONUType:         "HG8245H5",
		ServiceEnabled:  true,
		VLANMode:        "tag",
		ServiceType:     "internet",
		VLANID:          100,
		DownloadProfile: "100M",
		UploadProfile:   "100M",
		WANMode:         "wan_ip",
		WANIPMode:       "pppoe",
		VLANProfile:     "INTERNET",
		PPPoEUsername:   "example-user",
		PPPoEPassword:   "secret-password",
	}
}
