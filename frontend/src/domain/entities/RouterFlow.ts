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
  bytes: number;
}

export interface FlowStatus {
  asnDatasetUpdatedAt: string | null;
  asnCount: number;
}
