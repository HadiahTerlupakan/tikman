import type {
  Olt,
  CreateOltDto,
  UpdateOltDto,
  OltStats,
  UnconfiguredOnu,
  OltVlan,
  OltSystemSnapshot,
} from "../entities";

export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  getStats(id: string): Promise<OltStats>;
  getUnconfiguredOnus(id: string): Promise<UnconfiguredOnu[]>;
  getVlans(id: string): Promise<OltVlan[]>;
  getTcontProfiles(id: string): Promise<string[]>;
  getVlanProfiles(id: string): Promise<string[]>;
  getOnuTypes(id: string): Promise<string[]>;
  getSystem(id: string): Promise<OltSystemSnapshot>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
