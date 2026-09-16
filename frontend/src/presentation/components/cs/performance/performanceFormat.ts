import type { PerformancePeriod, WaitEndReason } from "@/domain/entities";

/** Minutes as the page shows them: "4,2 mnt", "2 j 5 mnt", or "—" when there
 * was nothing to measure. The switch to hours happens before rounding could
 * print "60 mnt". */
export function formatMinutes(minutes: number | null | undefined): string {
  if (minutes === null || minutes === undefined) {
    return "—";
  }
  const tenths = Math.round(minutes * 10) / 10;
  if (tenths < 60) {
    return `${tenths.toLocaleString("id-ID")} mnt`;
  }
  const whole = Math.round(minutes);
  const hours = Math.floor(whole / 60);
  const rest = whole % 60;
  return rest === 0 ? `${hours} j` : `${hours} j ${rest} mnt`;
}

export function formatPercent(pct: number | null | undefined): string {
  if (pct === null || pct === undefined) {
    return "—";
  }
  return `${Math.round(pct)}%`;
}

/** How a wait ended, in the words the inbox already uses. */
export const END_REASON_LABELS: Record<WaitEndReason, string> = {
  replied: "Dibalas",
  phone: "Dibalas dari HP",
  closed: "Ditutup tanpa balasan",
  abandoned: "Ditinggal",
};

const PERIOD_DATE_OPTIONS: Intl.DateTimeFormatOptions = {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "Asia/Jakarta",
};

/** The period a report covers, the way a person reads it: one date for a
 * single day, an en-dash range otherwise. A page switching periods (or
 * waiting on a custom range with nothing picked yet) still shows the old
 * figures while this changes underneath them — read in WIB, the same zone
 * the report's days are drawn in, so this never disagrees with them. */
export function formatPeriod(period: PerformancePeriod): string {
  const from = new Date(period.from).toLocaleDateString(
    "id-ID",
    PERIOD_DATE_OPTIONS,
  );
  if (period.from === period.to) {
    return from;
  }
  const to = new Date(period.to).toLocaleDateString(
    "id-ID",
    PERIOD_DATE_OPTIONS,
  );
  return `${from} – ${to}`;
}
