export type SwitchVendor = "crs" | "huawei_ce" | "fiberhome";
export type SwitchProtocol = "telnet" | "ssh" | "rest";
export type SwitchStatus = "unknown" | "up" | "down";

export interface ManagedSwitch {
  id: string;
  siteId: string | null;
  name: string;
  host: string;
  vendor: SwitchVendor;
  model: string | null;
  version: string | null;
  snmpPort: number;
  protocol: SwitchProtocol;
  port: number;
  username: string;
  hostKeyFingerprint: string;
  presentedFingerprint: string | null;
  hostKeyChanged: boolean;
  enabled: boolean;
  status: SwitchStatus;
  lastSeenAt: string | null;
  writesEnabled: boolean;
  rackPosition: number | null;
  createdAt: string;
  updatedAt: string;
}

export interface SwitchInterface {
  id: string;
  switchId: string;
  name: string;
  alias: string;
  /** rj, sfp, qsfp or virtual. */
  kind: "rj" | "sfp" | "qsfp" | "virtual";
  ifIndex: number | null;
  adminUp: boolean;
  operStatus: string;
  speedMbps: number | null;
  mode: string;
  pvid: number | null;
  allowedVlans: string;
  stpEdge: boolean | null;
  lastInBps: number | null;
  lastOutBps: number | null;
  inErrors: number | null;
  outErrors: number | null;
  inDiscards: number | null;
  lastPolledAt: string | null;
}

export interface CreateSwitchDto {
  name: string;
  host: string;
  vendor: SwitchVendor;
  snmpCommunity: string;
  snmpPort?: number;
  protocol: SwitchProtocol;
  port?: number;
  username: string;
  password: string;
  /** The SSH key the admin confirmed in the connection test. */
  hostKeyFingerprint?: string;
}

export interface UpdateSwitchDto {
  name?: string;
  host?: string;
  snmpCommunity?: string;
  snmpPort?: number;
  protocol?: SwitchProtocol;
  port?: number;
  username?: string;
  password?: string;
  rackPosition?: number;
  clearRackPosition?: boolean;
}

/** A test either identifies the switch or, over SSH, shows a key nobody has confirmed yet. */
export type SwitchConnectionTestResult =
  | { kind: "identified"; model: string; version: string }
  | { kind: "unpinned"; presentedFingerprint: string };

export interface SwitchVlan {
  switchId: string;
  vid: number;
  name: string;
}

export interface SwitchNeighbor {
  id: string;
  switchId: string;
  port: string;
  kind: "lldp" | "mac";
  mac: string;
  vid: number | null;
  remoteName: string;
  remotePort: string;
  remotePlatform: string;
}

export interface SwitchBackupSummary {
  id: string;
  takenAt: string;
  /** daily or manual. */
  reason: string;
  size: number;
}

export interface SwitchBackup extends SwitchBackupSummary {
  switchId: string;
  /** Config text with secrets removed. */
  masked: string;
}
