export type RouterStatus = "unknown" | "up" | "down";
export type RouterApiStatus =
  | "unknown"
  | "ok"
  | "error"
  | "certificate_changed";
export type RouterApiProtocol = "rest" | "api-ssl" | "api";
export type BgpState = "established" | "down";

/** One item of the router's /system health, as MIKROTIK-MIB's gauge table reports it. */
export interface RouterHealthItem {
  name: string;
  value: number;
  /** MIKROTIK-MIB unit: 1 °C, 2 RPM, 3 dV, 4 dA, 5 dW, 6 status (0 ok, 1 fail). */
  unit: number;
}

export type RouterRole = "border" | "bras" | "other";

export interface BrasCore {
  cpu: string;
  load: number;
}

/** total and free are null when the pool's ranges are not plain IPv4. */
export interface BrasPool {
  name: string;
  used: number;
  total: number | null;
  free: number | null;
}

export interface BrasRadius {
  id: string;
  address: string;
  requests: number;
  accepts: number;
  rejects: number;
  timeouts: number;
  pending: number;
  rttMs: number | null;
  /** null until there is a previous snapshot to judge against. */
  answering: boolean | null;
}

export interface BrasUser {
  name: string;
  downBps: number;
  upBps: number;
}

export interface BrasSnapshot {
  sessions: number;
  cores: BrasCore[];
  pools: BrasPool[];
  radius: BrasRadius[];
  topUsers: BrasUser[];
  queuesFound: boolean;
}

/** One minute of a BRAS, as the page charts it. */
export interface BrasSample {
  time: string;
  sessions: number;
  cpuMax: number;
  cpuAvg: number;
  radiusAccepts: number;
  radiusRejects: number;
  radiusTimeouts: number;
  poolMinFree: number | null;
}

/** A minute in which sessions fell sharply. */
export interface BrasDrop {
  time: string;
  lost: number;
}

/** The BRAS page's data; the hourly counts are null when history cannot answer them. */
export interface RouterBrasView {
  bras: BrasSnapshot | null;
  brasAt: string | null;
  samples: BrasSample[];
  sessions15mAgo: number | null;
  radiusAccepts1h: number | null;
  radiusRejects1h: number | null;
  radiusTimeouts1h: number | null;
  drops: BrasDrop[];
}

export interface MikrotikRouter {
  id: string;
  name: string;
  host: string;
  siteId: string | null;
  snmpPort: number;
  apiUsername: string;
  apiPort: number;
  apiProtocol: RouterApiProtocol;
  enabled: boolean;
  tlsFingerprint: string;
  presentedFingerprint: string | null;
  status: RouterStatus;
  apiStatus: RouterApiStatus;
  apiError: string | null;
  cpuLoad: number | null;
  memoryTotal: number | null;
  memoryFree: number | null;
  uptimeSeconds: number | null;
  rosVersion: string | null;
  boardName: string | null;
  lastSeenAt: string | null;
  flowEnabled: boolean;
  flowSampling: number;
  flowLastSeenAt: string | null;
  bgpEstablished: number;
  bgpTotal: number;
  /** Enabled interfaces that receive nothing although they carried traffic. */
  silentInterfaces: number;
  rackPosition: number | null;
  role: RouterRole;
  roleSetByAdmin: boolean;
  roleGuess: RouterRole | null;
  /** null until a BRAS has been read once. */
  bras: BrasSnapshot | null;
  brasAt: string | null;
  uplinkInterfaceId: string | null;
  /** null until the router's gauge table has been read once. */
  health: RouterHealthItem[] | null;
  healthAt: string | null;
}

export interface RouterInterface {
  id: string;
  routerId: string;
  name: string;
  comment: string;
  type: string;
  vlanId: number | null;
  parentName: string | null;
  ifIndex: number | null;
  speedMbps: number | null;
  disabled: boolean;
  operStatus: string;
  addresses: string;
  lastInBps: number | null;
  lastOutBps: number | null;
  lastPolledAt: string | null;
  lastTrafficAt: string | null;
  /** Set while the interface receives nothing although the router reports it up. */
  silentSince: string | null;
}

export interface RouterBgpSession {
  id: string;
  routerId: string;
  name: string;
  remoteAddress: string;
  remoteAs: number;
  localAddress: string;
  state: BgpState;
  uptimeSeconds: number;
  prefixCount: number;
  lastStartedAt: string | null;
  lastStoppedAt: string | null;
  interfaceId: string | null;
  updatedAt: string;
}

export interface RouterBgpEvent {
  id: string;
  routerId: string;
  sessionName: string;
  fromState: BgpState;
  toState: BgpState;
  occurredAt: string;
}

export interface RouterTrafficPoint {
  time: string;
  inBps: number;
  outBps: number;
}

export interface TimeRange {
  from: string;
  to: string;
}

/**
 * A graph's range as the operator chose it: a preset counts back from now and
 * follows the clock, a span picked by hand stays where it was put.
 */
export type TrafficRange =
  | { kind: "relative"; hours: number }
  | { kind: "absolute"; from: string; to: string };

export interface TestRouterConnectionDto {
  host: string;
  snmpCommunity: string;
  snmpPort?: number;
  apiUsername: string;
  apiPassword: string;
  apiPort?: number;
  apiProtocol?: RouterApiProtocol;
  tlsFingerprint?: string;
}

export interface CreateRouterDto extends TestRouterConnectionDto {
  name: string;
  role?: RouterRole;
}

export interface UpdateRouterDto {
  name?: string;
  host?: string;
  snmpCommunity?: string;
  snmpPort?: number;
  apiUsername?: string;
  apiPassword?: string;
  apiPort?: number;
  apiProtocol?: RouterApiProtocol;
  enabled?: boolean;
  flowEnabled?: boolean;
  flowSampling?: number;
  rackPosition?: number;
  clearRackPosition?: boolean;
  role?: RouterRole;
  resetRole?: boolean;
}

export interface RouterConnectionTestResult {
  snmpError?: string;
  apiError?: string;
  fingerprint?: string;
  boardName?: string;
  version?: string;
}
