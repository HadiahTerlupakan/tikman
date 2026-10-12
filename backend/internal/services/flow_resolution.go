package services

import (
	"fmt"
	"time"
)

// maxFlowPoints bounds one series in a response: more points than a chart
// has pixels only costs the query and the browser.
const maxFlowPoints = 1500

// flowSource is a relation the flow API reads: the raw minutes or a rollup,
// with the step its rows are spaced at and how long it keeps them.
type flowSource struct {
	table      string
	timeColumn string
	step       time.Duration
	retention  time.Duration // 0 keeps rows forever
}

// flowSources, finest first. Retention matches migration 58's policies.
var flowSources = []flowSource{
	{table: "router_flow_minute", timeColumn: "time", step: time.Minute, retention: 7 * 24 * time.Hour},
	{table: "router_flow_5min", timeColumn: "bucket", step: 5 * time.Minute, retention: 90 * 24 * time.Hour},
	{table: "router_flow_hourly", timeColumn: "bucket", step: time.Hour, retention: 2 * 365 * 24 * time.Hour},
	{table: "router_flow_daily", timeColumn: "bucket", step: 24 * time.Hour},
}

// flowResolution is where a query reads and the step it reports at.
type flowResolution struct {
	source flowSource
	step   time.Duration
}

// resolveFlowRange picks the finest source whose retention still holds from
// — span alone is not enough, a short range a month back has no raw minutes
// left — then widens the step, in whole multiples of the source's, until the
// range fits maxFlowPoints buckets.
func resolveFlowRange(from, to, now time.Time) flowResolution {
	source := flowSources[len(flowSources)-1]
	for _, s := range flowSources {
		if s.retention == 0 || now.Sub(from) <= s.retention {
			source = s
			break
		}
	}
	// maxFlowPoints-1 intervals: a range not aligned to the step touches one
	// bucket more than it spans.
	need := to.Sub(from) / (maxFlowPoints - 1)
	multiple := int64((need + source.step - 1) / source.step)
	if multiple < 1 {
		multiple = 1
	}
	return flowResolution{source: source, step: source.step * time.Duration(multiple)}
}

// flowZone is the zone router_flow_daily buckets its days in (migration 58).
const flowZone = "Asia/Jakarta"

// bucketExpr is the time_bucket call that widens the source's rows to the
// step, with the one interval it binds. Daily rows start at 17:00 UTC, so
// widening them in UTC would split every WIB day across two buckets; the finer
// sources are whole-hour aligned and need no zone.
func (r flowResolution) bucketExpr() (expr, interval string) {
	if r.source.table == "router_flow_daily" {
		return "time_bucket(?::interval, " + r.source.timeColumn + ", '" + flowZone + "')",
			fmt.Sprintf("%d days", int64(r.step/r.source.step))
	}
	return "time_bucket(?::interval, " + r.source.timeColumn + ")",
		fmt.Sprintf("%d seconds", int64(r.step.Seconds()))
}
