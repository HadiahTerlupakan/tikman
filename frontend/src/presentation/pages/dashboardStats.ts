import { OltStatus, OntStatus } from "@/domain/entities";
import type { Olt, Ont } from "@/domain/entities";

export interface OltSummary {
  total: number;
  online: number;
  offline: number;
  error: number;
}

export interface OntSummary {
  /** Rows the server returned. The buckets below describe exactly these. */
  counted: number;
  /** What the server says exists. Exceeds counted when the page hit its cap. */
  total: number;
  online: number;
  offline: number;
  los: number;
  dyingGasp: number;
  unknown: number;
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

// reportedTotal comes from the list response, which the API caps at 500 rows.
// Counting the rows instead would understate a network larger than the cap
// without ever saying so.
export function summariseOnts(
  onts: Ont[] | undefined,
  reportedTotal?: number,
): OntSummary {
  const list = Array.isArray(onts) ? onts : [];
  const count = (status: OntStatus) =>
    list.filter((o) => o.status === status).length;

  return {
    counted: list.length,
    total: reportedTotal ?? list.length,
    online: count(OntStatus.ONLINE),
    offline: count(OntStatus.OFFLINE),
    los: count(OntStatus.LOS),
    dyingGasp: count(OntStatus.DYING_GASP),
    unknown: count(OntStatus.UNKNOWN),
  };
}

/** True when the buckets describe only part of the network. */
export function isPartialSummary(summary: OntSummary): boolean {
  return summary.total > summary.counted;
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

// The OLT list is the authority for which OLTs exist: an OLT with no ONTs still
// belongs on the board, and an ONT pointing at an OLT that is not in the list
// would be a row nobody could act on.
export function summariseByOlt(
  olts: Olt[] | undefined,
  onts: Ont[] | undefined,
): OltBreakdown[] {
  const ontList = Array.isArray(onts) ? onts : [];

  const rows = (Array.isArray(olts) ? olts : []).map((olt) => {
    const owned = ontList.filter((ont) => ont.oltId === olt.id);
    const online = owned.filter(
      (ont) => ont.status === OntStatus.ONLINE,
    ).length;

    return {
      oltId: olt.id,
      oltName: olt.name,
      oltStatus: olt.status,
      ontTotal: owned.length,
      online,
      impaired: owned.length - online,
      availability: uptimePercent(online, owned.length),
    };
  });

  return rows.sort(compareByNeedForAttention);
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

// Only online ONTs qualify. An offline ONT reports whatever it last measured
// before it went dark, and those stale readings are the most negative in the
// set — they would fill the list and hide the ONTs that are still up and
// degrading, which are the ones worth a site visit.
export function weakestSignals(
  onts: Ont[] | undefined,
  limit = 5,
): WeakSignal[] {
  const list = Array.isArray(onts) ? onts : [];

  return list
    .filter(
      (ont): ont is Ont & { rxPower: number } =>
        ont.status === OntStatus.ONLINE && typeof ont.rxPower === "number",
    )
    .sort((a, b) => a.rxPower - b.rxPower)
    .slice(0, limit)
    .map((ont) => ({
      id: ont.id,
      name: ont.name || ont.serialNumber,
      serialNumber: ont.serialNumber,
      oltName: ont.oltName,
      rxPower: ont.rxPower,
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
