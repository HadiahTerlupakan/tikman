package services

import "time"

// TrafficUsage is the data an ONU moved over a window, measured from the OLT's
// cumulative counters.
type TrafficUsage struct {
	DownloadBytes uint64 `json:"download_bytes"`
	UploadBytes   uint64 `json:"upload_bytes"`
}

// gponLineRateBps is the downstream line rate of a GPON PON, used as the
// ceiling for what one ONU could physically have moved in a bucket. The
// upstream is half that, so the same figure is a safe bound for both.
const gponLineRateBps = 2_488_320_000

// trafficUsageOver sums the counter's forward movement across the window.
//
// Two discontinuities are excluded rather than counted. A counter that goes
// backwards means the ONU restarted. A jump larger than the link could carry
// in the elapsed time is not traffic either: replacing an ONU, or the day this
// system started reading the right OID, moves the counter by hundreds of
// gigabytes in one step and would otherwise be billed as usage.
func trafficUsageOver(rows []ONTMetricsRow, step time.Duration) TrafficUsage {
	ceiling := plausibleBytes(step)

	var usage TrafficUsage
	var prevRx, prevTx *uint64

	for i := range rows {
		usage.UploadBytes += counterAdvance(prevRx, rows[i].FirstRxBytes, rows[i].LastRxBytes, ceiling)
		usage.DownloadBytes += counterAdvance(prevTx, rows[i].FirstTxBytes, rows[i].LastTxBytes, ceiling)
		if rows[i].LastRxBytes != nil {
			prevRx = rows[i].LastRxBytes
		}
		if rows[i].LastTxBytes != nil {
			prevTx = rows[i].LastTxBytes
		}
	}

	return usage
}

// plausibleBytes is the most one ONU could move in a bucket, used to tell
// traffic from a counter discontinuity.
func plausibleBytes(step time.Duration) uint64 {
	if step <= 0 {
		return 0
	}
	return uint64(step.Seconds() * gponLineRateBps / 8)
}

// counterAdvance measures one bucket's contribution: the traffic since the
// previous bucket's last reading, plus the movement within this one. A step
// beyond what the link could carry is dropped, not clamped: the true figure is
// unknown, and a ceiling reported as fact would be worse than a gap.
func counterAdvance(previous, first, last *uint64, ceiling uint64) uint64 {
	if first == nil || last == nil || *last < *first {
		return 0
	}

	advance := *last - *first
	if ceiling > 0 && advance > ceiling {
		return 0
	}

	if previous != nil && *first >= *previous {
		gap := *first - *previous
		if ceiling == 0 || gap <= ceiling {
			advance += gap
		}
	}
	return advance
}
