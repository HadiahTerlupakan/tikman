export enum OltProtocol {
  SSH = "ssh",
  TELNET = "telnet",
}

export enum OltStatus {
  ONLINE = "online",
  OFFLINE = "offline",
  ERROR = "error",
}

export enum OltModel {
  ZTE_C300 = "zte_c300",
  ZTE_C320 = "zte_c320",
  HSGQ = "hsgq",
}

// One source of truth for the model select and table column, so adding a
// vendor touches a single place. The HSGQ hint tells the operator its readings
// rest on a third-party OID reference, not the vendor's own MIB.
export const OLT_MODELS: { value: OltModel; label: string; hint?: string }[] = [
  { value: OltModel.ZTE_C300, label: "ZTE C300" },
  { value: OltModel.ZTE_C320, label: "ZTE C320" },
  { value: OltModel.HSGQ, label: "HSGQ", hint: "OID belum diverifikasi" },
];

export interface Olt {
  id: string;
  siteId: string;
  siteName: string;
  name: string;
  ipAddress: string;
  model: OltModel;
  preferredProtocol: OltProtocol;
  username: string;
  snmpCommunity: string;
  sshPort: number;
  telnetPort: number;
  snmpPort: number;
  status: OltStatus;
  lastSeen: string | null;
  // Physical location for SNMP OID calculation (ZTE C300)
  rack: number;
  shelf: number;
  slot: number;
  createdAt: string;
  updatedAt: string;
}

export interface CreateOltDto {
  siteId: string;
  name: string;
  ipAddress: string;
  model: OltModel;
  preferredProtocol: OltProtocol;
  username: string;
  password: string;
  snmpCommunity?: string;
  sshPort?: number;
  telnetPort?: number;
  snmpPort?: number;
  // Physical location for SNMP OID calculation (ZTE C300)
  rack?: number;
  shelf?: number;
  slot?: number;
}

export interface UpdateOltDto {
  siteId?: string;
  name?: string;
  ipAddress?: string;
  model?: OltModel;
  preferredProtocol?: OltProtocol;
  username?: string;
  password?: string;
  snmpCommunity?: string;
  sshPort?: number;
  telnetPort?: number;
  snmpPort?: number;
  // Physical location for SNMP OID calculation (ZTE C300)
  rack?: number;
  shelf?: number;
  slot?: number;
}

export interface OltStats {
  totalOnts: number;
  ontsWithMetrics: number;
  percentage: number;
  lastPollTime?: string;
  oltId?: string;
}
