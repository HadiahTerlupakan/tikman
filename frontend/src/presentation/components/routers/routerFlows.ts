import type {
  FlowDestinations,
  MikrotikRouter,
  TrafficRange,
} from "@/domain/entities";
import { resolveTrafficRange } from "@/application/hooks/trafficRange";
import type { ChartRow } from "./routerTraffic";

const HOURS_PER_DAY = 24;
const MINUTE_MS = 60_000;

/** The POP filter value for addresses in no POP's blocks. */
export const OTHER_POP = "other";

/** Must match the backend's topIPRetention. */
export const TOP_IPS_RETENTION_MS = 7 * HOURS_PER_DAY * 60 * MINUTE_MS;

/** flowd marks a router heard every minute; five silent minutes is a stop. */
export const FLOW_STALE_MS = 5 * MINUTE_MS;

export const FLOW_RANGE_PRESETS = [
  { label: "15 menit", value: 0.25 },
  { label: "1 jam", value: 1 },
  { label: "24 jam", value: HOURS_PER_DAY },
  { label: "7 hari", value: 7 * HOURS_PER_DAY },
  { label: "30 hari", value: 30 * HOURS_PER_DAY },
  { label: "90 hari", value: 90 * HOURS_PER_DAY },
  { label: "1 tahun", value: 365 * HOURS_PER_DAY },
];

export const DEFAULT_FLOW_RANGE: TrafficRange = { kind: "relative", hours: 1 };

/** The organisation with its number, since two ASes can share a name. */
export function asnLabel(asn: number, name: string): string {
  if (asn === 0) return name || "Lainnya";
  return name ? `${name} (AS${asn})` : `AS${asn}`;
}

export function popLabel(
  id: string | null,
  labels: Map<string, string>,
): string {
  if (id === null) return "Lainnya (di luar blok POP)";
  return labels.get(id) ?? id;
}

/**
 * One row per time with a column per drawn series, zero where it had none.
 * Pivoted by timestamp: the server omits empty buckets, so two series in one
 * response need not share times and array index means nothing.
 */
export function flowChartRows(data: FlowDestinations | undefined): {
  rows: ChartRow[];
  series: string[];
} {
  if (!data) return { rows: [], series: [] };
  const drawn = data.series.filter((s) => s.points.length > 0);
  const names = drawn.map((s) => asnLabel(s.asn, s.name));
  const emptyRow = (time: string): ChartRow => {
    const row: ChartRow = { time };
    for (const name of names) row[name] = 0;
    return row;
  };
  const byTime = new Map<string, ChartRow>();
  drawn.forEach((s, i) => {
    for (const point of s.points) {
      const row = byTime.get(point.time) ?? emptyRow(point.time);
      row[names[i]] = point.bps;
      byTime.set(point.time, row);
    }
  });
  const rows = [...byTime.values()].sort(
    (a, b) => Date.parse(String(a.time)) - Date.parse(String(b.time)),
  );
  return { rows, series: names };
}

export function topIpsAvailable(
  range: TrafficRange,
  now: Date = new Date(),
): boolean {
  const { from } = resolveTrafficRange(range, now);
  return now.getTime() - Date.parse(from) <= TOP_IPS_RETENTION_MS;
}

export function flowStale(router: MikrotikRouter, nowMs: number): boolean {
  if (!router.flowEnabled) return false;
  if (router.flowLastSeenAt === null) return true;
  return nowMs - Date.parse(router.flowLastSeenAt) > FLOW_STALE_MS;
}

const SECONDS_PER_MINUTE = 60;
const SECONDS_PER_HOUR = 3_600;
const SECONDS_PER_DAY = 86_400;

/** The step a summary's peak was measured over, in words. */
export function peakWindowLabel(seconds: number): string {
  if (seconds >= SECONDS_PER_DAY) return `${seconds / SECONDS_PER_DAY} hari`;
  if (seconds >= SECONDS_PER_HOUR) return `${seconds / SECONDS_PER_HOUR} jam`;
  return `${seconds / SECONDS_PER_MINUTE} menit`;
}
