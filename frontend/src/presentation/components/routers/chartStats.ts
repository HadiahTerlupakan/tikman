import type { ChartRow } from "./routerTraffic";

export interface SeriesStats {
  name: string;
  current: number | null;
  average: number | null;
  maximum: number | null;
}

/**
 * Cacti's legend for each line: its latest sample, its mean and its peak over
 * the rows shown. A row without the series (a gap in polling, or a series that
 * only starts partway) is skipped rather than read as zero, which would drag
 * the mean down for a link that was never measured then. "Current" is read
 * from the last row only: a link that stopped reporting has no current value,
 * and showing its last good rate beside live ones would hide that.
 */
export function chartStats(data: ChartRow[], series: string[]): SeriesStats[] {
  const last = data[data.length - 1];
  return series.map((name) => {
    const latest = last?.[name];
    const current =
      typeof latest === "number" && Number.isFinite(latest) ? latest : null;
    let sum = 0;
    let count = 0;
    let maximum: number | null = null;
    for (const row of data) {
      const value = row[name];
      if (typeof value !== "number" || !Number.isFinite(value)) continue;
      sum += value;
      count += 1;
      if (maximum === null || value > maximum) maximum = value;
    }
    return {
      name,
      current,
      average: count > 0 ? sum / count : null,
      maximum,
    };
  });
}
