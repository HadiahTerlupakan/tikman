package connectivity

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The configure form reopened with "Use VEIP" off however the ONU was set up,
// because the running config was never read for it. Saving from that form then
// dropped the setting.
const veipConfig = `interface gpon-olt_1/3/1
  onu 15 type ALL sn HWTCB403E8A0
interface gpon-onu_1/3/1:15
  tcont 1 name bridge profile 1G
  gemport 1 name bridge tcont 1
  service-port 1 vport 1 user-vlan 214 vlan 214
pon-onu-mng gpon-onu_1/3/1:15
  service bridge gemport 1 vlan 214
  vlan port veip_1 mode tag vlan 214
`

func TestParseZTEONUServicesReadsVEIP(t *testing.T) {
	services := ParseZTEONUServices(veipConfig)

	service, ok := services[ONTLocation{Slot: 3, Port: 1, ONTID: 15}]
	require.True(t, ok)
	assert.True(t, service.UseVEIP)
	assert.Equal(t, 214, service.VLANID)
}

// An ONU configured without the VEIP line must not come back with it on, or
// reopening the form would turn it on for everyone.
func TestParseZTEONUServicesLeavesVEIPOffWhenAbsent(t *testing.T) {
	withoutVEIP := `interface gpon-olt_1/3/1
  onu 15 type ALL sn HWTCB403E8A0
pon-onu-mng gpon-onu_1/3/1:15
  service bridge gemport 1 vlan 214
`

	service := ParseZTEONUServices(withoutVEIP)[ONTLocation{Slot: 3, Port: 1, ONTID: 15}]

	assert.False(t, service.UseVEIP)
}
