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
  /** ISO code of the country, "" when unknown; see countrySource. */
  country: string;
  /**
   * "server" when IPinfo located the address; "as" when only the AS's
   * registered country is known, for ranges written before flowd located
   * addresses.
   */
  countrySource: "server" | "as" | "";
  bytes: number;
  /** Sampled flow records over the range; 0 for windows that predate it. */
  flows: number;
  /** The network's type in PeeringDB, "" when it states none. */
  asnType: string;
  /** From free lists: a Tor exit, a VPN network or a hosting network. */
  anonymity: "tor" | "vpn" | "hosting" | "";
}

export interface FlowStatus {
  asnDatasetUpdatedAt: string | null;
  asnCount: number;
}

export type RouteAdviceStatus =
  | "transit_only"
  | "inbound_via_transit"
  | "shorter_outbound"
  | "optimal";

/** The best candidate one BGP peer offers for a prefix. */
export interface RoutePath {
  peer: string;
  remoteAs: number;
  transit: boolean;
  asPath: string;
  pathLength: number;
  localPref: number;
  /** The peer's input filter chain, where a rule preferring this path goes. */
  inputFilter: string;
}

/** How much of a prefix's AS's download arrived from one peer. */
export interface RouteInbound {
  peer: string;
  transit: boolean;
  bytes: number;
}

/** One busy destination prefix with how it is routed and what to check. */
export interface RouteAdvice {
  prefix: string;
  asn: number;
  name: string;
  downloadBytes: number;
  uploadBytes: number;
  status: RouteAdviceStatus;
  active: RoutePath | null;
  alternatives: RoutePath[] | null;
  inbound: RouteInbound[] | null;
  /** The peer a shorter_outbound prefix could leave by; "" otherwise. */
  suggested: string;
  checkedAt: string;
}
