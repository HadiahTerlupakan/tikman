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
func TestReadZTEConfigSnapshotNeedsABulkRead(t *testing.T) {
	_, err := ReadZTEConfigSnapshot(context.Background(), &scriptedCommander{})
	if !errors.Is(err, ErrUnsupported) {
		t.Fatalf("err = %v, want ErrUnsupported", err)
	}
}

// Verbatim from the C300. The registration command takes one of these names;
// the model an ONU reports over OMCI — F609V9 for the F609 below — is not one
// of them and the OLT rejects it.
const onuTypeListing = `onu-type ZTEG-F609 gpon description F609
onu-type ZTEG-F660 gpon description F660
onu-type HG8245H5 gpon description Huawei
  onu-type ZTEG-F609 gpon description duplicate
`

func TestParseZTEONUTypesListsWhatTheOLTAccepts(t *testing.T) {
	names := parseZTEONUTypes(onuTypeListing)

	want := []string{"HG8245H5", "ZTEG-F609", "ZTEG-F660"}
	if len(names) != len(want) {
		t.Fatalf("got %v, want %v", names, want)
	}
	for i, name := range names {
		if name != want[i] {
			t.Errorf("type %d = %q, want %q", i, name, want[i])
		}
	}
}
