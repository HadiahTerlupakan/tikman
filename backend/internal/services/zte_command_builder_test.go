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
		WANMode:         "pppoe",
		VLANProfile:     "INTERNET",
		PPPoEUsername:   "example-user",
		PPPoEPassword:   "secret-password",
	}

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.NoError(t, err)
	require.NotContains(t, strings.Join(commands, "\n"), req.PPPoEPassword)
	require.Equal(t, []string{
		"configure terminal",
		"interface gpon-olt_1/3/1",
		"onu 7 type HG8245H5 sn HWTCB403E8A0",
		"exit",
		"interface gpon-onu_1/3/1:7",
		"name 258179206252-Saraswati",
		"tcont 1 name internet profile-name 100M",
		"gemport 1 name internet tcont 1",
		"service-port 1 vport 1 user-vlan 100 vlan 100",
		"wan-ip 1 mode pppoe username example-user password <redacted> vlan-profile INTERNET",
		"exit",
		"commit",
	}, commands)
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

func TestBuildZTEGPONRegisterCommandsRejectsInvalidServiceMode(t *testing.T) {
	req := validZTECommandRequest()
	req.WANMode = "ipoe"

	commands, err := BuildZTEGPONRegisterCommands(req, 7)
	require.Error(t, err)
	require.Nil(t, commands)
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
		WANMode:         "pppoe",
		VLANProfile:     "INTERNET",
		PPPoEUsername:   "example-user",
		PPPoEPassword:   "secret-password",
	}
}
