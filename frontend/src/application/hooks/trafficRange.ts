import type { TimeRange, TrafficRange } from "@/domain/entities";

const HOUR_MS = 3_600_000;

export const DEFAULT_TRAFFIC_RANGE: TrafficRange = {
  kind: "relative",
  hours: 6,
};

/** Resolves a range to the span to fetch, a relative one counting back from now. */
export function resolveTrafficRange(
  range: TrafficRange,
  now: Date = new Date(),
): TimeRange {
  if (range.kind === "absolute") return { from: range.from, to: range.to };
  return {
    from: new Date(now.getTime() - range.hours * HOUR_MS).toISOString(),
    to: now.toISOString(),
  };
}

/**
 * A relative range is keyed on its length, not its endpoints: the cache entry
 * stays put across renders while each refetch reads up to a new now.
 */
export function trafficRangeKey(range: TrafficRange): (string | number)[] {
  return range.kind === "relative"
    ? ["last", range.hours]
    : ["between", range.from, range.to];
}

/** Only a range that follows the clock has anything new to fetch. */
export function liveRefetchInterval(
  range: TrafficRange,
  intervalMs: number,
): number | false {
  return range.kind === "relative" ? intervalMs : false;
}
