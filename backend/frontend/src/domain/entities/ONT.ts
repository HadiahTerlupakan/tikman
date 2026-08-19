export type ONTStatus = 'online' | 'offline' | 'los' | 'unknown';

export interface ONT {
  id: string;
  oltId: string;
  oltName: string;
  portId: number;
  ontId: number;
  serialNumber: string;
  description: string;
  status: ONTStatus;
  lastSeenAt: string | null;
  createdAt: string;
  updatedAt: string;
  rxPower?: number | null;
  txPower?: number | null;
  distance?: number;
}

export interface CreateONTPayload {
  oltId: string;
  portId: number;
  ontId: number;
  serialNumber: string;
  description?: string;
  status?: ONTStatus;
}

export interface UpdateONTPayload {
  description?: string;
  status?: ONTStatus;
}
