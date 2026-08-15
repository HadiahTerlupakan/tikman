export enum OntStatus {
  ONLINE = "online",
  OFFLINE = "offline",
  LOS = "los",
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
  rxPower: number;
  txPower: number;
  temperature: number;
  voltage: number;
  distance: number;
  rxBytes: number;
  txBytes: number;
}
