export interface ChassisEntity {
  index: number;
  description: string;
  class: number;
  serial?: string;
  software?: string;
}

export interface OltSystemInfo {
  description: string;
  name: string;
  uptimeSeconds: number;
  entities: ChassisEntity[];
}

export type OltPortKind = "pon" | "uplink" | "other";

export interface OltPort {
  ifIndex: number;
  name: string;
  kind: OltPortKind;
  rack: number;
  slot: number;
  port: number;
  adminUp: boolean;
  operUp: boolean;
  adminStatus: number;
  operStatus: number;
}

export interface OltCard {
  rack: number;
  shelf: number;
  slot: number;
  type: string;
}

export interface CardHealth {
  slot: number;
  temperatureC?: number;
  cpuPercent?: number;
  memoryPercent?: number;
}

export interface ZteOnuType {
  name: string;
  pon: string;
  description?: string;
  maxTcont?: number;
  maxGemport?: number;
}

export interface TcontProfile {
  name: string;
  type: number;
  fixedBwKbps: number;
  assuredBwKbps: number;
  maxBwKbps: number;
}

export interface OltSystemSnapshot {
  system: OltSystemInfo;
  ports: OltPort[];
  cards: OltCard[];
  cardHealth: CardHealth[];
  onuTypes: ZteOnuType[];
  speedProfiles: TcontProfile[];
  updatedAt?: string;
}
