import type { OltStatus } from "./Olt";

/** How many ONTs are in each state, across every OLT. */
export interface OntStatusCounts {
  total: number;
  online: number;
  offline: number;
  los: number;
  dyingGasp: number;
  unknown: number;
}

/** One row of the per-OLT table. */
export interface OltBreakdownRow {
  oltId: string;
  oltName: string;
  oltStatus: OltStatus;
  ontTotal: number;
  online: number;
  /** Every ONT that is not online, whatever the reason. */
  impaired: number;
}

/** One of the worst optical readings currently being received. */
export interface WeakSignalReading {
  id: string;
  name: string;
  serialNumber: string;
  oltName: string;
  rxPower: number;
}

/**
 * The overview page's figures, counted by the database.
 *
 * The page used to fetch a page of ONTs and count them here, which described
 * only as many as one request returned: a 651-ONT chassis showed 221.
 */
export interface DashboardStats {
  onts: OntStatusCounts;
  olts: OltBreakdownRow[];
  weakestSignals: WeakSignalReading[];
}
