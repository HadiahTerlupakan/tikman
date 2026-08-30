import { OltStatus } from "@/domain/entities";
import type {
  Olt,
  OltBreakdownRow,
  WeakSignalReading,
} from "@/domain/entities";

export interface OltSummary {
  total: number;
  online: number;
  offline: number;
  error: number;
}

export function summariseOlts(olts: Olt[] | undefined): OltSummary {
  const list = Array.isArray(olts) ? olts : [];
  return {
    total: list.length,
    online: list.filter((o) => o.status === OltStatus.ONLINE).length,
    offline: list.filter((o) => o.status === OltStatus.OFFLINE).length,
    error: list.filter((o) => o.status === OltStatus.ERROR).length,
  };
}

export interface OltBreakdown {
  oltId: string;
  oltName: string;
  oltStatus: OltStatus;
  ontTotal: number;
  online: number;
  /** Every ONT that is not online, whatever the reason. */
  impaired: number;
  availability: number | null;
}

// The server counts every ONT; ranking and the availability percentage stay
// here because they are how the table is read, not what it contains.
export function rankOltRows(
  rows: OltBreakdownRow[] | undefined,
): OltBreakdown[] {
  const list = Array.isArray(rows) ? rows : [];

  return list
    .map((row) => ({
      ...row,
      availability: uptimePercent(row.online, row.ontTotal),
    }))
    .sort(compareByNeedForAttention);
}

// Worst availability first: the point of the table is to answer "where do I
// look", so an OLT with nothing to measure sorts last rather than as 0%.
function compareByNeedForAttention(a: OltBreakdown, b: OltBreakdown): number {
  if (a.availability === null || b.availability === null) {
    if (a.availability === b.availability) {
      return a.oltName.localeCompare(b.oltName);
    }
    return a.availability === null ? 1 : -1;
  }
  if (a.availability !== b.availability) {
    return a.availability - b.availability;
  }
  return a.oltName.localeCompare(b.oltName);
}

export interface WeakSignal {
  id: string;
  name: string;
  serialNumber: string;
  oltName: string;
  rxPower: number;
}

// The OLT labels an ONU inconsistently, so a reading with no name falls back to
// the serial, which is what a technician matches against the box in the field.
export function toWeakSignals(
  readings: WeakSignalReading[] | undefined,
): WeakSignal[] {
  return (Array.isArray(readings) ? readings : []).map((reading) => ({
    ...reading,
    name: reading.name || reading.serialNumber,
  }));
}

// GPON Class B+ optics are specified down to -27 dBm, so a link below that is
// running on margin that weather or a dirty connector will finish off. -24 is
// the point where it stops being comfortable.
const RX_MARGINAL_DBM = -27;
const RX_WATCH_DBM = -24;

export function signalTone(rxPower: number): "danger" | "warning" | "success" {
  if (rxPower < RX_MARGINAL_DBM) {
    return "danger";
  }
  if (rxPower < RX_WATCH_DBM) {
    return "warning";
  }
  return "success";
}

// Returns null when there is nothing to measure so callers render a dash rather
// than 0%, which would read as a total outage.
export function uptimePercent(online: number, total: number): number | null {
  if (total === 0) {
    return null;
  }
  return Math.round((online / total) * 100);
}

// GPON deployments treat sustained availability below ~95% as worth attention,
// and below 80% as an outage rather than degradation.
const AVAILABILITY_WARN = 95;
const AVAILABILITY_CRITICAL = 80;

export function availabilityTone(
  percent: number | null,
): "success" | "warning" | "danger" | "neutral" {
  if (percent === null) {
    return "neutral";
  }
  if (percent < AVAILABILITY_CRITICAL) {
    return "danger";
  }
  if (percent < AVAILABILITY_WARN) {
    return "warning";
  }
  return "success";
}

const MINUTE_MS = 60_000;
const HOUR_MS = 3_600_000;

// Seconds are kept because the page refreshes every 15s: rounding a fresh poll
// up to "1m ago" would make live data look stale.
export function formatAge(elapsedMs: number): string {
  if (elapsedMs < MINUTE_MS) {
    return `${Math.max(Math.floor(elapsedMs / 1000), 0)}s ago`;
  }
  if (elapsedMs < HOUR_MS) {
    return `${Math.floor(elapsedMs / MINUTE_MS)}m ago`;
  }
  return `${Math.floor(elapsedMs / HOUR_MS)}h ago`;
}
