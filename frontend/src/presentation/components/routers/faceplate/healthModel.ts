import type { MikrotikRouter, RouterHealthItem } from "@/domain/entities";
import { routerReachable } from "./portModel";

export interface HealthReading {
  name: string;
  label: string;
  text: string;
  failed: boolean;
}

export interface HealthView {
  /** Only a fresh reading may light an LED; anything else is last-known. */
  fresh: boolean;
  psus: { n: number; ok: boolean }[];
  failures: string[];
  readings: HealthReading[];
}

// The gauge walk runs every minute, so five minutes without one means it
// stopped answering, and a stuck reading must not keep a PSU looking healthy.
const FRESH_MS = 5 * 60 * 1000;

const UNIT_CELSIUS = 1;
const UNIT_RPM = 2;
const UNIT_STATUS = 6;
const STATUS_FAIL = 1;
// MIKROTIK-MIB reports volts, amps and watts in tenths.
const TENTHS_SUFFIX: Record<number, string> = { 3: "V", 4: "A", 5: "W" };

const PSU_STATE = /^psu(\d+)-state$/;

const LABELS: [RegExp, (m: RegExpMatchArray) => string][] = [
  [/^cpu-temperature$/, () => "Suhu CPU"],
  [/^board-temperature(\d+)$/, (m) => `Suhu board ${m[1]}`],
  [/^fan(\d+)-speed$/, (m) => `Kipas ${m[1]}`],
  [/^psu(\d+)-voltage$/, (m) => `PSU${m[1]} tegangan`],
  [/^power-consumption$/, () => "Daya"],
];

function label(name: string): string {
  for (const [re, make] of LABELS) {
    const m = name.match(re);
    if (m) return make(m);
  }
  return name;
}

function text(item: RouterHealthItem): string {
  if (item.unit === UNIT_CELSIUS) return `${item.value} °C`;
  if (item.unit === UNIT_RPM) return `${item.value} RPM`;
  if (item.unit === UNIT_STATUS)
    return item.value === STATUS_FAIL ? "fail" : "ok";
  const suffix = TENTHS_SUFFIX[item.unit];
  return suffix ? `${(item.value / 10).toFixed(1)} ${suffix}` : `${item.value}`;
}

const failed = (item: RouterHealthItem) =>
  item.unit === UNIT_STATUS && item.value === STATUS_FAIL;

function fresh(r: MikrotikRouter, now: Date): boolean {
  if (!r.healthAt || !routerReachable(r)) return false;
  return now.getTime() - new Date(r.healthAt).getTime() <= FRESH_MS;
}

function psuNumber(item: RouterHealthItem): number | null {
  const m = item.unit === UNIT_STATUS ? item.name.match(PSU_STATE) : null;
  return m ? Number(m[1]) : null;
}

/** What the router's last health reading says, and whether it may be shown as live. */
export function healthView(r: MikrotikRouter, now: Date): HealthView {
  const items = r.health ?? [];
  const psus = items
    .flatMap((item) => {
      const n = psuNumber(item);
      return n === null ? [] : [{ n, ok: !failed(item) }];
    })
    .sort((a, b) => a.n - b.n);
  const failures = items.filter(failed).map((item) => {
    const n = psuNumber(item);
    return n === null ? item.name : `PSU${n}`;
  });
  const readings = items.map((item) => ({
    name: item.name,
    label: label(item.name),
    text: text(item),
    failed: failed(item),
  }));
  return { fresh: fresh(r, now), psus, failures, readings };
}
