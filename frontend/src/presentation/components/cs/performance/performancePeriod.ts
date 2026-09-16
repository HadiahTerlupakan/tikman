import type { PerformancePeriod } from "@/domain/entities";

export type PeriodPreset = "hari-ini" | "7-hari" | "bulan-ini" | "bulan-lalu";

const WIB_OFFSET_MS = 7 * 60 * 60 * 1000;
const DAY_MS = 24 * 60 * 60 * 1000;

/** The WIB calendar day a moment falls on, as a UTC midnight so the arithmetic
 * below has no zone of its own. The report's days are WIB days on the server,
 * and a browser somewhere else must still ask for the same ones. */
function wibDay(moment: Date): Date {
  const shifted = new Date(moment.getTime() + WIB_OFFSET_MS);
  return new Date(
    Date.UTC(
      shifted.getUTCFullYear(),
      shifted.getUTCMonth(),
      shifted.getUTCDate(),
    ),
  );
}

function iso(day: Date): string {
  return day.toISOString().slice(0, 10);
}

function addDays(day: Date, days: number): Date {
  return new Date(day.getTime() + days * DAY_MS);
}

/** The dates a preset covers, counted from now. */
export function presetPeriod(
  preset: PeriodPreset,
  now: Date,
): PerformancePeriod {
  const today = wibDay(now);
  const firstOfThisMonth = Date.UTC(
    today.getUTCFullYear(),
    today.getUTCMonth(),
    1,
  );
  switch (preset) {
    case "hari-ini":
      return { from: iso(today), to: iso(today) };
    case "7-hari":
      return { from: iso(addDays(today, -6)), to: iso(today) };
    case "bulan-ini":
      return { from: iso(new Date(firstOfThisMonth)), to: iso(today) };
    case "bulan-lalu":
      return {
        from: iso(
          new Date(
            Date.UTC(today.getUTCFullYear(), today.getUTCMonth() - 1, 1),
          ),
        ),
        to: iso(addDays(new Date(firstOfThisMonth), -1)),
      };
  }
}
