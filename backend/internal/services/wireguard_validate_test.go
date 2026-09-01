package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestValidateAllowedIPsRejectsDefaultRoute(t *testing.T) {
	err := ValidateAllowedIPs([]string{"0.0.0.0/0"}, nil, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "default route")
}

func TestValidateAllowedIPsRejectsOverlapWithAnotherSite(t *testing.T) {
	others := []PeerNetwork{
		{PeerID: uuid.New(), SiteName: "Site Bandung", AllowedIPs: []string{"192.168.1.0/24"}},
	}
	err := ValidateAllowedIPs([]string{"192.168.1.128/25"}, others, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "Site Bandung",
		"the message must name the conflicting site, since identical private subnets across sites are the common case")
}

func TestValidateAllowedIPsRejectsOverlapWithTunnelSubnet(t *testing.T) {
	err := ValidateAllowedIPs([]string{"10.88.0.0/25"}, nil, "10.88.0.0/24", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tunnel subnet")
}

func TestValidateAllowedIPsRejectsOverlapWithReservedSubnet(t *testing.T) {
	err := ValidateAllowedIPs([]string{"172.18.0.0/16"}, nil, "10.88.0.0/24", []string{"172.16.0.0/12"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "reserved")
}

func TestValidateAllowedIPsRejectsEmptyAndMalformed(t *testing.T) {
	require.Error(t, ValidateAllowedIPs(nil, nil, "10.88.0.0/24", nil))
	require.Error(t, ValidateAllowedIPs([]string{"10.10.10.5"}, nil, "10.88.0.0/24", nil))
}

func TestValidateAllowedIPsAcceptsDistinctSubnets(t *testing.T) {
	others := []PeerNetwork{
		{PeerID: uuid.New(), SiteName: "Site Bandung", AllowedIPs: []string{"192.168.1.0/24"}},
	}
	require.NoError(t, ValidateAllowedIPs([]string{"10.10.10.0/24", "192.168.88.0/24"}, others, "10.88.0.0/24", nil))
}

func TestValidateTunnelAddress(t *testing.T) {
	require.NoError(t, ValidateTunnelAddress("10.88.0.5", "10.88.0.0/24", "10.88.0.1", []string{"10.88.0.4"}))

	err := ValidateTunnelAddress("10.99.0.5", "10.88.0.0/24", "10.88.0.1", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "outside")

	err = ValidateTunnelAddress("10.88.0.1", "10.88.0.0/24", "10.88.0.1", nil)
	require.Error(t, err)
	require.Contains(t, err.Error(), "server")

	err = ValidateTunnelAddress("10.88.0.4", "10.88.0.0/24", "10.88.0.1", []string{"10.88.0.4"})
	require.Error(t, err)
	require.Contains(t, err.Error(), "already")
}

func TestValidateKeepalive(t *testing.T) {
	require.NoError(t, ValidateKeepalive(25))
	require.Error(t, ValidateKeepalive(5))
	require.Error(t, ValidateKeepalive(200))
}

func TestValidateEndpointHost(t *testing.T) {
	for _, ok := range []string{"1.2.3.4", "vpn.contoh.id", "a-b.example.co.id", "2001:db8::1"} {
		require.NoError(t, ValidateEndpointHost(ok), ok)
	}
	for _, bad := range []string{
		"1.2.3.4;/user/add name=hax password=hax group=full;",
		"vpn.contoh.id ",
		"vpn contoh id",
		"-leadinghyphen.example",
		"",
	} {
		require.Error(t, ValidateEndpointHost(bad), bad)
	}
}

func TestValidateSiteLabelRefusesLineBreaks(t *testing.T) {
	require.NoError(t, ValidateSiteLabel("POP Cikarang"))
	require.Error(t, ValidateSiteLabel("Site A\nPostUp = sh"))
	require.Error(t, ValidateSiteLabel("Site\tA"))
}
