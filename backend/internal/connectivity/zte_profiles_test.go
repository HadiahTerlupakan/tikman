package connectivity

import (
	"context"
	"errors"
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
