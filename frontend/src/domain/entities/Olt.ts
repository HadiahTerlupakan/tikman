export enum OltProtocol {
  SSH = "ssh",
  TELNET = "telnet",
}

export enum OltStatus {
  ONLINE = "online",
  OFFLINE = "offline",
  ERROR = "error",
}

export interface Olt {
  id: string;
  siteId: string;
  siteName: string;
  name: string;
  ipAddress: string;
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
