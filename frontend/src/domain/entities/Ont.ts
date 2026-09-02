export enum OntStatus {
  ONLINE = "online",
  OFFLINE = "offline",
  LOS = "los",
  DYING_GASP = "dying_gasp",
  UNKNOWN = "unknown",
}

export interface Ont {
  id: string;
  oltId: string;
  oltName: string;
  slot?: number;
  portId: number;
  ontId: number;
  serialNumber: string;
  name: string; // ← Add name field
  description: string;
  /** The subscriber's WhatsApp number in 628xxx form; ties an incoming chat to this ONT. */
  phone?: string;
  status: OntStatus;
  deviceType?: string;
  hardwareVersion?: string;
  softwareVersion?: string;
  ipAddress?: string;
  macAddress?: string;
  /** Where this subscriber's drop lands. Operator knowledge; the OLT has none. */
  odpId?: string;
  odpPort?: number;
  lastSeenAt: string | null;
  createdAt: string;
  updatedAt: string;
  distance?: number;
  rxPower?: number | null;
  txPower?: number | null;
}

export interface CreateOntDto {
  oltId: string;
  portId: number;
  ontId: number;
  serialNumber: string;
  description?: string;
  status?: OntStatus;
  phone?: string;
}

export interface UpdateOntDto {
  description?: string;
  status?: OntStatus;
  phone?: string;
}

export interface OntMetrics {
  time: string;
  /** null when the ONT reported no optical signal (distinct from a real 0 dBm) */
  rxPower: number | null;
  /** null when the ONT reported no optical signal (distinct from a real 0 dBm) */
  txPower: number | null;
  temperature: number;
  voltage: number;
  txBiasCurrent: number | null;
  distance: number;
  rxBytes: number;
  txBytes: number;
  rxPackets: number;
  txPackets: number;
  rxErrors: number;
  txErrors: number;
  // null when the OLT model exposes no per-ONU rate gauges, which is different
  // from a measured 0 Mbps. The UI renders null as "-".
  rxMbps?: number | null;
  txMbps?: number | null;
  rxMaxMbps?: number;
  txMaxMbps?: number;
}

// Topology response types for OLT slot/port/ONT structure
export interface TopologySlotResponse {
  slot: number;
  ports?: TopologyPortResponse[];
}

export interface TopologyPortResponse {
  portId: number;
  onts?: TopologyOntResponse[];
}

export interface TopologyOntResponse {
  portId: number;
  ontId: number;
  serialNumber: string;
  runState: number;
  name?: string;
  description?: string;
  rxPower?: number | null;
  txPower?: number | null;
  distance?: number;
  status?: string;
  lastSeenAt?: string | null;
}

export interface TrafficUsage {
  downloadBytes: number;
  uploadBytes: number;
}

/** A consolidated traffic series with the volume moved over its window. */
export interface TrafficSeries {
  points: OntMetrics[];
  usage: TrafficUsage;
}

/** Summed traffic of every ONU under an OLT, or under one of its PON ports. */
export interface AggregateTrafficPoint {
  time: string;
  rxMbps: number;
  txMbps: number;
  rxMaxMbps: number;
  txMaxMbps: number;
  onlineOnts: number;
}

/** TroubledSummary covers every matching ONT, not only the page returned. */
export interface TroubledSummary {
  ontCount: number;
  totalDownMinutes: number;
}

/** TroubledOnt is one subscriber and how much trouble it has been in. */
export interface TroubledOnt {
  ontId: string;
  serialNumber: string;
  name: string;
  oltName: string;
  /**
   * The line card. Port numbers repeat across cards, so narrowing to a PON
   * needs both. A row discovery could not place carries 0, the same card the
   * topology groups it under, so it matches there and on no other card.
   */
  slot: number;
  portId: number;
  ontNumber: number;
  status: string;
  trapCount: number;
  downMinutes: number;
}

/** TroubledResult is the ranked page and the picture it is a page of. */
export interface TroubledResult {
  data: TroubledOnt[];
  summary: TroubledSummary;
}
