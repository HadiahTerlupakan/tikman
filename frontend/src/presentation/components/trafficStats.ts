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
