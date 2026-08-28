package connectivity

import (
	"context"
	"errors"
	"strings"
	"testing"
)

type scriptedCommander struct {
	outputs  map[string]string
	failures map[string]string
	sent     []string
	err      error
}

func (c *scriptedCommander) ExecuteCommand(_ context.Context, cmd string) (*CommandResult, error) {
	c.sent = append(c.sent, cmd)
	if c.err != nil {
		return nil, c.err
	}
	return &CommandResult{Output: c.outputs[cmd], Error: c.failures[cmd], Success: c.failures[cmd] == ""}, nil
}

func (c *scriptedCommander) BatchExecute(context.Context, []string) ([]*CommandResult, error) {
	return nil, errors.New("not used")
}

// Verbatim from a C300 V2.1.0, trailing spaces and all.
const tcontListing = `show gpon profile tcont
Profile name :default  
 Type           FBW(kbps)   ABW(kbps)   MBW(kbps)   PRIORITY   WEIGHT    
 1              10000       0           0           N/A         N/A         
 
Profile name :1G  
 Type           FBW(kbps)   ABW(kbps)   MBW(kbps)   PRIORITY   WEIGHT    
 3              0           512         1024000     N/A         N/A         
 
BRAS-PANCORANMAS-DPK#`

func TestReadZTETcontProfiles(t *testing.T) {
	commander := &scriptedCommander{outputs: map[string]string{zteTcontProfileCommand: tcontListing}}

	profiles, err := ReadZTETcontProfiles(context.Background(), commander)
	if err != nil {
		t.Fatalf("ReadZTETcontProfiles: %v", err)
	}

	want := []string{"default", "1G"}
	if len(profiles) != len(want) {
		t.Fatalf("got %v, want %v", profiles, want)
	}
	for i, name := range profiles {
		if name != want[i] {
			t.Errorf("profile %d = %q, want %q", i, name, want[i])
		}
	}

	// Paging has to be off first, or a long listing stalls waiting for a keypress.
	if len(commander.sent) != 2 || commander.sent[0] != disablePagingCommand {
		t.Errorf("sent %v, want paging disabled before the listing", commander.sent)
	}
}

func TestReadZTETcontProfilesRejectsACLIError(t *testing.T) {
	commander := &scriptedCommander{
		outputs:  map[string]string{zteTcontProfileCommand: "%Error 20200: Invalid input detected"},
		failures: map[string]string{zteTcontProfileCommand: "Invalid input detected"},
	}

	if _, err := ReadZTETcontProfiles(context.Background(), commander); err == nil {
		t.Fatal("a CLI error must not be reported as an empty profile list")
	}
}

func TestParseZTEProfileNamesIgnoresRepeats(t *testing.T) {
	names := parseZTEProfileNames("Profile name :A\nProfile name :A\nProfile name :B\n")

	if len(names) != 2 || names[0] != "A" || names[1] != "B" {
		t.Fatalf("got %v, want [A B]", names)
	}
}

// Verbatim shape of the wan-ip lines a C300 running config carries. The
// password field is why the raw config must never leave the reader.
const runningConfigExtract = `
pon-onu-mng gpon-onu_1/3/1:1
  wan-ip 1 mode pppoe username 258179206252 password secret vlan-profile PPPOE-21
pon-onu-mng gpon-onu_1/3/1:2
  wan-ip 1 mode pppoe username 258170473762 password secret vlan-profile PPPOE-21
pon-onu-mng gpon-onu_1/3/2:1
  wan-ip 1 mode pppoe username 251230727315 password secret vlan-profile PPPOE-214
pon-onu-mng gpon-onu_1/3/2:2
  wan-ip 1 mode pppoe username 258164843870 password secret vlan-profile PPP
`

// Verbatim from the C300, wrapped by the CLI at its 80th column: the name
// PPPOE-214 is split across two lines, and different username lengths move the
// break, so a naive read invented PPPOE-21, PPPOE-2 and PPP as separate
// profiles. Every line below is exactly 80 characters before the break.
var wrappedRunningConfig = strings.Join([]string{
	"pon-onu-mng gpon-onu_1/3/1:1",
	"  service ServiceName gemport 2 vlan 214",
	"  wan-ip 2 mode pppoe username 258179206252 password 12345 vlan-profile PPPOE-21",
	"4 host 2",
	"pon-onu-mng gpon-onu_1/3/1:10",
	"  wan-ip 1 mode pppoe username 2581692447 password 12345 vlan-profile PPPOE-214 h",
	"ost 1",
}, "\n")

func TestRankZTEVLANProfilesRejoinsWrappedLines(t *testing.T) {
	names := rankZTEVLANProfiles(wrappedRunningConfig)

	if len(names) != 1 || names[0] != "PPPOE-214" {
		t.Fatalf("got %v, want [PPPOE-214]: the CLI wrap must not split the name", names)
	}
}

// The profile the OLT is standardised on has to lead: PPPOE-214 and PPP are a
// handful of one-off entries next to 185 uses of PPPOE-21 on the real device.
func TestRankZTEVLANProfilesOrdersByUse(t *testing.T) {
	names := rankZTEVLANProfiles(runningConfigExtract)

	want := []string{"PPPOE-21", "PPP", "PPPOE-214"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("name %d = %q, want %q", i, name, want[i])
		}
	}
}

func TestRankZTEVLANProfilesIsEmptyWithoutWanIP(t *testing.T) {
	if names := rankZTEVLANProfiles("interface gpon-onu_1/3/1:1\n  tcont 1 profile default\n"); len(names) != 0 {
		t.Fatalf("got %v, want none", names)
	}
}

// A commander with no bulk read cannot fetch the running config, and must say
// so rather than report that the OLT has no profiles.
func TestReadZTEVLANProfilesNeedsABulkRead(t *testing.T) {
	_, err := ReadZTEVLANProfiles(context.Background(), &scriptedCommander{})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}
