export type FlowDirection = "download" | "upload";

/** Narrows a flow read; pop "other" is traffic of addresses in no POP's blocks. */
export interface FlowFilter {
  pop?: string;
  upstream?: string;
  direction?: FlowDirection;
}

export interface FlowPoint {
  time: string;
  bps: number;
}

/** One destination AS; ASN 0 is "Lainnya", an empty name is an unnamed AS. */
export interface FlowSeries {
  asn: number;
  name: string;
  points: FlowPoint[];
}

export interface FlowDestinations {
  stepSeconds: number;
  series: FlowSeries[];
}

export interface FlowUpstreamShare {
  upstreamInterfaceId: string;
  bytes: number;
}

export interface FlowSummaryRow {
  asn: number;
  name: string;
  bytes: number;
  percent: number;
  avgBps: number;
  peakBps: number;
  upstreams: FlowUpstreamShare[];
}

export interface FlowSummary {
  totalBytes: number;
  peakWindowSeconds: number;
  rows: FlowSummaryRow[];
}

/** One upstream's part of one destination over the range. */
export interface FlowPeeringRow {
  upstreamInterfaceId: string;
  bytes: number;
  percent: number;
  avgBps: number;
  peakBps: number;
}

export interface FlowPeeringSeries {
  upstreamInterfaceId: string;
  points: FlowPoint[];
}

/** One POP's part of a destination; a null POP is addresses in no POP's blocks. */
export interface FlowPeeringPop {
  popInterfaceId: string | null;
  bytes: number;
  percent: number;
  upstreams: FlowUpstreamShare[];
}

/** Which upstreams carry one destination AS, largest first. */
export interface FlowPeering {
  asn: number;
  name: string;
  totalBytes: number;
  stepSeconds: number;
  peakWindowSeconds: number;
  rows: FlowPeeringRow[];
  series: FlowPeeringSeries[];
  pops: FlowPeeringPop[];
}

export interface FlowAsnBytes {
  asn: number;
  name: string;
  bytes: number;
}

/** A null POP is the traffic of addresses in no POP's blocks. */
export interface FlowPopRow {
  popInterfaceId: string | null;
  bytes: number;
  topAsns: FlowAsnBytes[];
}

export interface FlowTopIp {
  ip: string;
  asn: number;
  name: string;
  /** ISO code of where the address is located (DB-IP), "" when unknown. */
  country: string;
  bytes: number;
}

export interface FlowStatus {
  asnDatasetUpdatedAt: string | null;
  asnCount: number;
}
