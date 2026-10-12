export type RouterStatus = "unknown" | "up" | "down";
export type RouterApiStatus =
  | "unknown"
  | "ok"
  | "error"
  | "certificate_changed";
export type BgpState = "established" | "down";

export interface MikrotikRouter {
  id: string;
  name: string;
  host: string;
  siteId: string | null;
  snmpPort: number;
  apiUsername: string;
  apiPort: number;
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
  bgpEstablished: number;
  bgpTotal: number;
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
  tlsFingerprint?: string;
}

export interface CreateRouterDto extends TestRouterConnectionDto {
  name: string;
  tlsFingerprint: string;
}

export interface UpdateRouterDto {
  name?: string;
  host?: string;
  snmpCommunity?: string;
  snmpPort?: number;
  apiUsername?: string;
  apiPassword?: string;
  apiPort?: number;
  enabled?: boolean;
}

export interface RouterConnectionTestResult {
  snmpError?: string;
  apiError?: string;
  fingerprint?: string;
  boardName?: string;
  version?: string;
}
