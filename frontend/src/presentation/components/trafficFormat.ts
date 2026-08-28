const UNITS = ["B", "KB", "MB", "GB", "TB", "PB"];
const STEP = 1024;

/**
 * Formats a byte counter for display. These are the OLT's lifetime totals, so
 * they reach hundreds of gigabytes and are unreadable as raw digits.
 */
export function formatBytes(bytes?: number | null): string {
  if (bytes === undefined || bytes === null || !Number.isFinite(bytes)) {
    return "—";
  }
  if (bytes <= 0) return "0 B";

  let value = bytes;
  let unit = 0;
  while (value >= STEP && unit < UNITS.length - 1) {
    value /= STEP;
    unit += 1;
  }

  return `${value.toFixed(unit === 0 ? 0 : 2)} ${UNITS[unit]}`;
}

/** Formats a rate in Mbps, dropping to Kbps below 1 Mbps so idle links read. */
export function formatRate(mbps?: number | null): string {
  if (mbps === undefined || mbps === null || !Number.isFinite(mbps)) {
    return "—";
  }
  if (mbps >= 1) return `${mbps.toFixed(2)} Mbps`;

  const kbps = mbps * 1000;
  return `${kbps.toFixed(kbps < 0.01 ? 4 : 2)} Kbps`;
}
