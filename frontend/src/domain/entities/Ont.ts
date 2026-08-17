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
  portId: number;
  ontId: number;
  serialNumber: string;
  description: string;
  status: OntStatus;
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
}

export interface UpdateOntDto {
  description?: string;
  status?: OntStatus;
}

export interface OntMetrics {
  time: string;
  /** null when the ONT reported no optical signal (distinct from a real 0 dBm) */
  rxPower: number | null;
  /** null when the ONT reported no optical signal (distinct from a real 0 dBm) */
  txPower: number | null;
  temperature: number;
  voltage: number;
  distance: number;
  rxBytes: number;
  txBytes: number;
}
