package connectivity

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// tcontListing lives in zte_profiles_test.go, captured verbatim from the same
// C300 listing this parses.

func TestParseZTETcontProfiles(t *testing.T) {
	profiles := parseZTETcontProfiles(tcontListing)

	assert.Equal(t, []ZTETcontProfile{
		{Name: "default", Type: 1, FixedBW: 10000, AssuredBW: 0, MaxBW: 0},
		{Name: "1G", Type: 3, FixedBW: 0, AssuredBW: 512, MaxBW: 1024000},
	}, profiles)
}

// A bandwidth row belongs to the profile it was printed under. Matching the
// whole listing at once would give every profile the first profile's figures.
func TestParseZTETcontProfilesDoesNotShareRowsBetweenProfiles(t *testing.T) {
	profiles := parseZTETcontProfiles(tcontListing)

	assert.NotEqual(t, profiles[0].MaxBW, profiles[1].MaxBW)
}
