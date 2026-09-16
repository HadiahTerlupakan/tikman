package services

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tikman/olt-provisioning/internal/models"
)

// endedWait is a wait that started and ended at chosen moments, sent whenever
// the customer sent it.
func endedWait(sent, started, ended time.Time) models.CSWait {
	reason := models.WaitReplied
	return models.CSWait{
		ID: uuid.New(), CustomerSentAt: sent, StartedAt: started,
		EndedAt: &ended, EndReason: &reason,
	}
}

// The figures must be checkable by hand against the same query in Postgres.
// These are what percentile_cont gives for these four numbers.
func TestPercentileInterpolatesLikePercentileCont(t *testing.T) {
	sorted := []float64{1.5, 10, 30, 600}

	assert.InDelta(t, 20.0, percentile(sorted, 0.5), 1e-9)
	assert.InDelta(t, 172.5, percentile(sorted, 0.75), 1e-9)
	assert.InDelta(t, 429.0, percentile(sorted, 0.9), 1e-9)
	assert.InDelta(t, 5.0, percentile([]float64{5}, 0.9), 1e-9)
}

func TestSummarizeMinutesCountsTheTargetInclusively(t *testing.T) {
	stats := summarizeMinutes([]float64{3, 15, 15.5})

	assert.Equal(t, 3, stats.Count)
	require.NotNil(t, stats.MedianMinutes)
	assert.InDelta(t, 15.0, *stats.MedianMinutes, 1e-9)
	require.NotNil(t, stats.P90Minutes)
	assert.InDelta(t, 15.4, *stats.P90Minutes, 1e-9)
	require.NotNil(t, stats.WithinTargetPct)
	assert.InDelta(t, 200.0/3, *stats.WithinTargetPct, 1e-9, "15 minutes exactly is on time")
}

// A day nobody was answered is not a day everyone was answered at once.
func TestSummarizeMinutesOfNothingHasNoFigures(t *testing.T) {
	stats := summarizeMinutes(nil)

	assert.Equal(t, 0, stats.Count)
	assert.Nil(t, stats.MedianMinutes)
	assert.Nil(t, stats.P90Minutes)
	assert.Nil(t, stats.WithinTargetPct)
}

func TestSystemDelayedIsMoreThanHalfAnHourLate(t *testing.T) {
	sent := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	assert.False(t, systemDelayed(endedWait(sent, sent.Add(30*time.Minute), sent.Add(time.Hour))))
	assert.True(t, systemDelayed(endedWait(sent, sent.Add(31*time.Minute), sent.Add(time.Hour))))
}

// A phone reply sent before the message reached TikMan would otherwise count as
// a negative wait.
func TestTeamMinutesNeverGoNegative(t *testing.T) {
	started := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	assert.InDelta(t, 0.0, teamMinutes(endedWait(started, started, started.Add(-time.Minute))), 1e-9)
	assert.InDelta(t, 5.0, teamMinutes(endedWait(started, started, started.Add(5*time.Minute))), 1e-9)
}

func TestSessionStartsBreakAtAPauseOfThreeHoursOrMore(t *testing.T) {
	base := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)
	replies := []time.Time{
		base,                                   // 08:00 WIB, the first of the day
		base.Add(30 * time.Minute),             // half an hour later, same stretch
		base.Add(3*time.Hour + 29*time.Minute), // 2h59m after that, still the same
		base.Add(6*time.Hour + 29*time.Minute), // exactly three hours later: a new one
	}

	starts := sessionStarts(replies)

	require.Len(t, starts, 4)
	assert.True(t, starts[0].Equal(base))
	assert.True(t, starts[1].Equal(base))
	assert.True(t, starts[2].Equal(base))
	assert.True(t, starts[3].Equal(replies[3]))
}

func TestCountedMinutesStartAtTheLaterOfTheWaitAndTheStretch(t *testing.T) {
	night := time.Date(2026, 9, 9, 16, 0, 0, 0, time.UTC)
	stretch := time.Date(2026, 9, 10, 1, 0, 0, 0, time.UTC)

	overnight := endedWait(night, night, stretch.Add(10*time.Minute))
	assert.InDelta(t, 10.0, countedMinutes(overnight, stretch), 1e-9, "the night is not the morning CS's")

	later := endedWait(stretch.Add(5*time.Minute), stretch.Add(5*time.Minute), stretch.Add(10*time.Minute))
	assert.InDelta(t, 5.0, countedMinutes(later, stretch), 1e-9)
}

func TestAReportRangeCoversWholeWIBDays(t *testing.T) {
	r, err := ReportRangeFromDates("2026-09-10", "2026-09-11")
	require.NoError(t, err)

	assert.True(t, r.From.Equal(time.Date(2026, 9, 9, 17, 0, 0, 0, time.UTC)), "00:00 WIB on the 10th")
	assert.True(t, r.To.Equal(time.Date(2026, 9, 11, 17, 0, 0, 0, time.UTC)), "00:00 WIB on the 12th")
	assert.Equal(t, []string{"2026-09-10", "2026-09-11"}, r.dates())
}

func TestAReportRangeRefusesWhatItCannotRead(t *testing.T) {
	cases := []struct {
		name     string
		from, to string
	}{
		{"not a date", "2026-9-10", "2026-09-10"},
		{"nothing at all", "", ""},
		{"ends before it starts", "2026-09-11", "2026-09-10"},
		{"longer than a year", "2026-01-01", "2027-01-02"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ReportRangeFromDates(c.from, c.to)

			assert.ErrorIs(t, err, ErrValidation)
		})
	}

	_, err := ReportRangeFromDates("2026-01-01", "2027-01-01")
	assert.NoError(t, err, "366 days is the longest report allowed")
}

func TestReportDateIsTheWIBCalendarDay(t *testing.T) {
	assert.Equal(t, "2026-09-10", reportDate(time.Date(2026, 9, 10, 16, 59, 0, 0, time.UTC)), "23:59 WIB")
	assert.Equal(t, "2026-09-11", reportDate(time.Date(2026, 9, 10, 17, 1, 0, 0, time.UTC)), "00:01 WIB")
}
