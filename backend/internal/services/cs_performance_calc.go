package services

import (
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/tikman/olt-provisioning/internal/models"
)

const (
	// csReplyTargetMinutes is the benchmark the team is measured against: an
	// answer within this many minutes counts as on time.
	csReplyTargetMinutes = 15
	// csSessionGap separates two stretches of one CS's work. Measured on
	// production, the pauses between a CS's own replies fall either under two
	// hours or over six, so three hours splits them cleanly.
	csSessionGap = 3 * time.Hour
	// csSystemDelay is how late a message may reach TikMan before the waiting is
	// the system's doing rather than anyone's slowness.
	csSystemDelay = 30 * time.Minute
	// csSessionLookback is how far before a report's first reply a CS's earlier
	// replies are read, to find where that stretch of work began. Nobody works a
	// whole day without a three-hour pause.
	csSessionLookback = 24 * time.Hour
	// csReportMaxDays bounds one report to a year.
	csReportMaxDays = 366
)

// WaitStats is how fast a set of waits was answered, in minutes. The figures are
// pointers because a set with nothing in it has no median, and reporting zero
// would read as "answered instantly".
type WaitStats struct {
	Count           int      `json:"count"`
	MedianMinutes   *float64 `json:"median_minutes"`
	P90Minutes      *float64 `json:"p90_minutes"`
	WithinTargetPct *float64 `json:"within_target_pct"`
}

// summarizeMinutes reduces a set of waits to the figures the report shows.
func summarizeMinutes(minutes []float64) WaitStats {
	stats := WaitStats{Count: len(minutes)}
	if len(minutes) == 0 {
		return stats
	}
	sorted := append([]float64(nil), minutes...)
	sort.Float64s(sorted)

	within := 0
	for _, m := range sorted {
		if m <= csReplyTargetMinutes {
			within++
		}
	}
	median := percentile(sorted, 0.5)
	p90 := percentile(sorted, 0.9)
	pct := 100 * float64(within) / float64(len(sorted))
	stats.MedianMinutes, stats.P90Minutes, stats.WithinTargetPct = &median, &p90, &pct
	return stats
}

// percentile interpolates linearly between the two nearest ranks, the way
// Postgres's percentile_cont does, so a figure here can be checked against a
// query run by hand. sorted must be ascending and hold at least one value.
func percentile(sorted []float64, p float64) float64 {
	position := p * float64(len(sorted)-1)
	lower := int(math.Floor(position))
	if lower+1 >= len(sorted) {
		return sorted[len(sorted)-1]
	}
	return sorted[lower] + (position-float64(lower))*(sorted[lower+1]-sorted[lower])
}

// systemDelayed marks a wait whose first message reached TikMan long after the
// customer sent it: nobody could answer what had not arrived.
func systemDelayed(w models.CSWait) bool {
	return w.StartedAt.Sub(w.CustomerSentAt) > csSystemDelay
}

// answered says whether a wait ended with the customer getting an answer.
func answered(w models.CSWait) bool {
	return w.EndReason != nil && (*w.EndReason == models.WaitReplied || *w.EndReason == models.WaitPhone)
}

// teamMinutes is how long the customer waited once TikMan held their message.
func teamMinutes(w models.CSWait) float64 {
	return minutesBetween(w.StartedAt, *w.EndedAt)
}

// countedMinutes is the part of a wait charged to the CS who answered it: from
// the later of the wait's start and the start of their stretch of work.
func countedMinutes(w models.CSWait, sessionStart time.Time) float64 {
	from := w.StartedAt
	if sessionStart.After(from) {
		from = sessionStart
	}
	return minutesBetween(from, *w.EndedAt)
}

func minutesBetween(from, to time.Time) float64 {
	return math.Max(0, to.Sub(from).Minutes())
}

// sessionStarts answers, for each of one CS's replies in ascending order, when
// the stretch of work it belongs to began: the first reply after a pause of
// csSessionGap or more.
func sessionStarts(replies []time.Time) []time.Time {
	starts := make([]time.Time, len(replies))
	for i, at := range replies {
		if i == 0 || at.Sub(replies[i-1]) >= csSessionGap {
			starts[i] = at
			continue
		}
		starts[i] = starts[i-1]
	}
	return starts
}

// reportDate is the WIB calendar day a moment falls on. The report's days are
// the days the team works, not UTC days.
func reportDate(t time.Time) string {
	return t.In(wib).Format(time.DateOnly)
}

// ReportRange is the span one report covers, half open: [From, To).
type ReportRange struct {
	From time.Time
	To   time.Time
}

// ReportRangeFromDates reads the two inclusive WIB dates the page sends into the
// span they cover.
func ReportRangeFromDates(from, to string) (ReportRange, error) {
	start, err := time.ParseInLocation(time.DateOnly, from, wib)
	if err != nil {
		return ReportRange{}, fmt.Errorf("%w: tanggal awal harus berformat YYYY-MM-DD", ErrValidation)
	}
	end, err := time.ParseInLocation(time.DateOnly, to, wib)
	if err != nil {
		return ReportRange{}, fmt.Errorf("%w: tanggal akhir harus berformat YYYY-MM-DD", ErrValidation)
	}
	if end.Before(start) {
		return ReportRange{}, fmt.Errorf("%w: tanggal akhir tidak boleh sebelum tanggal awal", ErrValidation)
	}
	r := ReportRange{From: start, To: end.AddDate(0, 0, 1)}
	if r.days() > csReportMaxDays {
		return ReportRange{}, fmt.Errorf("%w: rentang laporan paling panjang %d hari", ErrValidation, csReportMaxDays)
	}
	return r, nil
}

func (r ReportRange) days() int {
	return int(r.To.Sub(r.From).Hours() / 24)
}

// dates lists every WIB day the range covers, so a chart can show a day nothing
// happened on as a gap rather than leaving it out.
func (r ReportRange) dates() []string {
	var days []string
	for day := r.From; day.Before(r.To); day = day.AddDate(0, 0, 1) {
		days = append(days, reportDate(day))
	}
	return days
}
