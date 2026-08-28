package connectivity

import (
	"encoding/json"
	"strings"
	"testing"
)

// Verbatim shape from the C300, including the 80-column wrap that splits the
// VLAN profile name and the password field the parser must never carry out.
const onuRunningConfig = `interface gpon-onu_1/3/1:1
  name 258179206252-Saraswati
  tcont 1 name VLAN0214-PPP profile default
  tcont 2 name INET profile 1G
  gemport 1 tcont 1
  gemport 2 name INET tcont 2
  service-port 2 vport 2 user-vlan 214 vlan 214
!
interface gpon-onu_1/3/1:2
  tcont 1 name BRIDGE profile default
  gemport 1 tcont 1
  service-port 1 vport 1 user-vlan untagged vlan 100
!
pon-onu-mng gpon-onu_1/3/1:1
  service ServiceName gemport 2 vlan 214
  wan-ip 2 mode pppoe username 258179206252 password 12345 vlan-profile PPPOE-21
4 host 2
!
`

func TestParseZTEONUServicesReadsTheProvisionedService(t *testing.T) {
	services := ParseZTEONUServices(onuRunningConfig)

	got, ok := services[ONTLocation{Slot: 3, Port: 1, ONTID: 1}]
	if !ok {
		t.Fatalf("ONU 1/3/1:1 missing from %v", services)
	}

	want := ZTEONUService{
		VLANID: 214, VLANMode: "tag", ServiceType: "internet",
		// Resolved through gemport 2 -> tcont 2, not the first T-CONT on the ONU.
		TCONTProfile: "1G",
		WANMode:      "wan_ip", WANIPMode: "pppoe",
		VLANProfile: "PPPOE-214", PPPoEUsername: "258179206252",
		PPPoEPassword: "12345",
	}
	if got != want {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}
}

// An ONU with no wan-ip is carrying traffic only.
func TestParseZTEONUServicesReadsAnUntaggedBridge(t *testing.T) {
	services := ParseZTEONUServices(onuRunningConfig)

	got := services[ONTLocation{Slot: 3, Port: 1, ONTID: 2}]

	if got.VLANMode != "untag" || got.VLANID != 100 {
		t.Errorf("VLAN = %d/%s, want 100/untag", got.VLANID, got.VLANMode)
	}
	if got.ServiceType != "bridge" || got.WANMode != "setup_via_ont" {
		t.Errorf("got %s/%s, want bridge/setup_via_ont", got.ServiceType, got.WANMode)
	}
}

func TestParseZTEONUServicesReadsThePassword(t *testing.T) {
	got := ParseZTEONUServices(onuRunningConfig)[ONTLocation{Slot: 3, Port: 1, ONTID: 1}]

	if got.PPPoEPassword != "12345" {
		t.Errorf("password = %q, want it read so a reconfigure can resend it", got.PPPoEPassword)
	}
}

// The service is stored as JSON. The password must not ride along in it: it is
// encrypted into its own column, the way the OLT's own credentials are.
func TestZTEONUServiceJSONOmitsThePassword(t *testing.T) {
	encoded, err := json.Marshal(ZTEONUService{PPPoEUsername: "user", PPPoEPassword: "12345"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	if strings.Contains(string(encoded), "12345") {
		t.Fatalf("the encoded service carries the password: %s", encoded)
	}
}
