// The time axis of a traffic graph, which is derived from the window the
// operator picked rather than from the data that happens to be in it.

const AXIS_TICK_COUNT = 5;

export function getPeriodDomain(period: string): [number, number] | undefined {
  const value = Number(period.slice(0, -1));
  const unit = period.slice(-1);
  if (!Number.isFinite(value) || value <= 0) {
    return undefined;
  }

  const now = Date.now();
  if (unit === "h") {
    return [now - value * 60 * 60 * 1000, now];
  }
  if (unit === "d") {
    return [now - value * 24 * 60 * 60 * 1000, now];
  }
  return undefined;
}

// Recharts derives ticks from the data points, not from `domain`, so a range with
// data in only part of it would label just that part and hide how much of the
// selected window is empty. Spacing ticks across the domain keeps the axis honest.
export function getAxisTicks(
  domain: [number, number] | undefined,
): number[] | undefined {
  if (!domain) {
    return undefined;
  }
  const [start, end] = domain;
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start) {
    return undefined;
  }
  const step = (end - start) / (AXIS_TICK_COUNT - 1);
  return Array.from({ length: AXIS_TICK_COUNT }, (_, i) =>
    Math.round(start + step * i),
  );
}
