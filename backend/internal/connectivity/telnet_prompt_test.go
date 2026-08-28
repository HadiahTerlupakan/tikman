package connectivity

import "testing"

func TestDeviceHostnameIgnoresTheConfigurationContext(t *testing.T) {
	for response, want := range map[string]string{
		"\r\nBRAS-PANCORANMAS-DPK#":                    "BRAS-PANCORANMAS-DPK",
		"some banner\r\nBRAS-PANCORANMAS-DPK(config)#": "BRAS-PANCORANMAS-DPK",
		"OLT(config-if)#":                              "OLT",
		"OLT>":                                         "OLT",
		"":                                             "",
	} {
		if got := deviceHostname(response); got != want {
			t.Errorf("deviceHostname(%q) = %q, want %q", response, got, want)
		}
	}
}

// The C300 marks a wrapped line with '$', and the old read treated that as a
// prompt. It returned halfway through an echoed command, and the next command
// went out while the OLT was still writing — which is how a registration
// reached the device as "onu 15 type universalOnuType id".
func TestEndsWithDevicePromptIgnoresTheWrapMarker(t *testing.T) {
	wrapped := "BRAS-PANCORANMAS-DPK(config-if)#onu 15 type HG8245H5 sn HWTCB4$"

	if endsWithDevicePrompt(wrapped, "BRAS-PANCORANMAS-DPK") {
		t.Fatal("a wrapped line must not be read as the prompt")
	}
}

func TestEndsWithDevicePromptAcceptsEveryContext(t *testing.T) {
	for _, output := range []string{
		"Building configuration...\r\nBRAS-PANCORANMAS-DPK#",
		"%Info 20272: Enter configuration commands\r\nBRAS-PANCORANMAS-DPK(config)#",
		"BRAS-PANCORANMAS-DPK(gpon-onu-mng 1/3/1:1)#",
	} {
		if !endsWithDevicePrompt(output, "BRAS-PANCORANMAS-DPK") {
			t.Errorf("prompt not recognised in %q", output)
		}
	}
}

// Command output routinely contains these characters. Ending a read on one
// leaves the rest of the output to be read as the next command's answer.
func TestEndsWithDevicePromptIgnoresOutputText(t *testing.T) {
	for _, output := range []string{
		"OnuIndex   Admin State  OMCC State\r\n1/3/6:18    enable       disable      LOS",
		"  wan-ip 1 mode pppoe username user password pass vlan-profile PPPOE-214 host 2",
		"%Error 20200: Invalid input detected at '^' marker.",
		"another-switch#",
	} {
		if endsWithDevicePrompt(output, "BRAS-PANCORANMAS-DPK") {
			t.Errorf("output read as a prompt: %q", output)
		}
	}
}

// Without a learned hostname the read still has to stop somewhere, so it falls
// back to a prompt-shaped token.
func TestEndsWithDevicePromptFallsBackWithoutAHostname(t *testing.T) {
	if !endsWithDevicePrompt("OLT(config)#", "") {
		t.Error("a prompt-shaped last line should end the read")
	}
	if endsWithDevicePrompt("some output with # inside", "") {
		t.Error("text containing # is not a prompt")
	}
}
