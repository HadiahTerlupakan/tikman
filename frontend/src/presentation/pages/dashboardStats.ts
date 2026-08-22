import { OltStatus, OntStatus } from "@/domain/entities";
import type { Olt, Ont } from "@/domain/entities";

export interface OltSummary {
  total: number;
  online: number;
  offline: number;
  error: number;
}

export interface OntSummary {
  total: number;
  online: number;
  offline: number;
  los: number;
  dyingGasp: number;
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

export function summariseOnts(onts: Ont[] | undefined): OntSummary {
  const list = Array.isArray(onts) ? onts : [];
  return {
    total: list.length,
    online: list.filter((o) => o.status === OntStatus.ONLINE).length,
    offline: list.filter((o) => o.status === OntStatus.OFFLINE).length,
    los: list.filter((o) => o.status === OntStatus.LOS).length,
    dyingGasp: list.filter((o) => o.status === OntStatus.DYING_GASP).length,
  };
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
