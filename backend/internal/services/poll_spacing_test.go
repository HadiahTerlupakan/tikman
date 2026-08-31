package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/tikman/olt-provisioning/internal/models"
)

func TestSpacingForKeepsTheTierWhenTheChassisAnswersQuickly(t *testing.T) {
	// The installations here read a status table in well under a second, so the
	// tier is what decides and nothing is stretched.
	assert.Equal(t, StatusInterval,
		SpacingFor(models.PollKindStatus, 400*time.Millisecond))
	assert.Equal(t, MetricsInterval,
		SpacingFor(models.PollKindMetrics, 20*time.Second))
}

func TestSpacingForStretchesPastAChassisTheTierCannotHold(t *testing.T) {
	// A ZTE agent serves about 140 values a second, so a chassis of ten thousand
	// ONUs needs roughly 72 seconds for one status table. A 60-second tier would
	// then have the chassis polled with no gap at all, leaving its agent no time
	// for anything else and starving the other tiers behind it.
	assert.Equal(t, 144*time.Second,
		SpacingFor(models.PollKindStatus, 72*time.Second))
}

func TestSpacingForLeavesTheAgentHalfItsTime(t *testing.T) {
	// The bound is the agent's CPU, not the network: at most half of it is ours.
	took := 45 * time.Second
	assert.Equal(t, took*2, SpacingFor(models.PollKindStatus, took))
}

func TestSpacingForIsPerTier(t *testing.T) {
	// A slow status run must not stretch the metrics tier, which has its own
	// interval and its own measured duration.
	assert.Equal(t, MetricsInterval,
		SpacingFor(models.PollKindMetrics, 72*time.Second))
}

func TestSpacingForIgnoresAnUnmeasuredRun(t *testing.T) {
	assert.Equal(t, DiscoveryInterval, SpacingFor(models.PollKindDiscovery, 0))
}
