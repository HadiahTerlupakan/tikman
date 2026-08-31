package services

import "time"

// metricsSource is the relation a history query reads from.
//
// Three of them exist and until now only the first was ever read, which meant
// the graphs could show seven days however far back they were asked for, while
// the installation kept — and paid to keep — a year of hourly rollups nothing
// could reach.
type metricsSource string

const (
	sourceRaw    metricsSource = "ont_metrics"
	source5Min   metricsSource = "ont_metrics_5min"
	sourceHourly metricsSource = "ont_metrics_hourly"
)

// The retention each source is kept for, matching the policies on the database.
// Raw is deliberately treated as slightly shorter than its policy: retention
// drops whole chunks, so the oldest day of raw data disappears at a moment
// nothing here can predict, and reading a rollup that definitely holds the range
// beats reading raw that might not.
const (
	rawRetention     = 6 * 24 * time.Hour
	fiveMinRetention = 30 * 24 * time.Hour
)

// sourceForRange picks the finest source that still holds the whole range.
//
// Coarsening earlier than necessary would throw away resolution the operator
// asked for; coarsening later than necessary returns an empty chart, because
// the rows are gone. The choice follows the range's start, since that is the
// end that falls off retention.
func sourceForRange(startTime, now time.Time) metricsSource {
	age := now.Sub(startTime)

	switch {
	case age < rawRetention:
		return sourceRaw
	case age < fiveMinRetention:
		return source5Min
	default:
		return sourceHourly
	}
}

// aggregated reports whether the source stores buckets rather than samples, and
// therefore names its time column bucket and carries no voltage or distance.
func (s metricsSource) aggregated() bool {
	return s != sourceRaw
}
