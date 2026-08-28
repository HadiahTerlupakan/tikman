/**
 * The 95th percentile of a sample set, the figure capacity planning and
 * burst billing are based on: it discards the top 5% of samples, so a brief
 * spike does not set the number the whole month is judged by.
 *
 * Empty buckets are excluded by the caller; a period with no samples has no
 * percentile rather than one of zero.
 */
export function percentile95(values: number[]): number | undefined {
  const samples = values.filter((value) => Number.isFinite(value));
  if (samples.length === 0) return undefined;

  const sorted = [...samples].sort((a, b) => a - b);
  // Nearest-rank: the smallest value at or above 95% of the way through.
  const rank = Math.ceil(sorted.length * 0.95);
  return sorted[Math.min(rank, sorted.length) - 1];
}

interface RateScale {
  divisor: number;
  unit: string;
}

/**
 * Picks one unit for a whole axis from its largest value. Choosing per tick
 * produced an axis reading "3.6, 2.7, 1.8, 900K, 0K", where 900K sat below 1.8
 * and neither carried a unit.
 */
export function rateScaleFor(maxValue: number): RateScale {
  if (!Number.isFinite(maxValue) || maxValue <= 0) {
    return { divisor: 0.001, unit: "Kbps" };
  }
  if (maxValue >= 1000) return { divisor: 1000, unit: "Gbps" };
  if (maxValue >= 1) return { divisor: 1, unit: "Mbps" };
  return { divisor: 0.001, unit: "Kbps" };
}

/**
 * Formats an axis tick on a shared scale. Small values keep a decimal, because
 * rounding them to whole numbers put two identical labels on one axis.
 */
export function formatRateTick(value: number, scale: RateScale): string {
  const scaled = value / scale.divisor;
  if (scaled === 0) return "0";
  return scaled >= 10 ? scaled.toFixed(0) : scaled.toFixed(1);
}
