package services

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAllocateTunnelAddressTakesFirstFree(t *testing.T) {
	address, err := AllocateTunnelAddress("10.88.0.0/24", "10.88.0.1", nil)
	require.NoError(t, err)
	require.Equal(t, "10.88.0.2", address)
}

func TestAllocateTunnelAddressFillsGaps(t *testing.T) {
	address, err := AllocateTunnelAddress("10.88.0.0/24", "10.88.0.1", []string{"10.88.0.2", "10.88.0.4"})
	require.NoError(t, err)
	require.Equal(t, "10.88.0.3", address, "a released address must be reused before the range grows")
}

func TestAllocateTunnelAddressFailsWhenSubnetIsFull(t *testing.T) {
	taken := []string{}
	for i := 2; i <= 6; i++ {
		taken = append(taken, fmt.Sprintf("10.88.0.%d", i))
	}
	_, err := AllocateTunnelAddress("10.88.0.0/29", "10.88.0.1", taken)
	require.Error(t, err)
	require.Contains(t, err.Error(), "no free address")
}

func TestSuggestAllowedIPsDerivesSlash24(t *testing.T) {
	require.Equal(t, []string{"10.10.10.0/24"}, SuggestAllowedIPs([]string{"10.10.10.5"}))
}

func TestSuggestAllowedIPsDeduplicates(t *testing.T) {
	got := SuggestAllowedIPs([]string{"10.10.10.5", "10.10.10.9", "192.168.88.2"})
	require.Equal(t, []string{"10.10.10.0/24", "192.168.88.0/24"}, got)
}

func TestSuggestAllowedIPsIgnoresUnusableAddresses(t *testing.T) {
	require.Empty(t, SuggestAllowedIPs([]string{"", "not-an-ip"}))
}
