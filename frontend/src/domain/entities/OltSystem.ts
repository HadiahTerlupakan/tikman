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

export interface OltSystemSnapshot {
  system: OltSystemInfo;
  ports: OltPort[];
  cards: OltCard[];
  updatedAt?: string;
}
