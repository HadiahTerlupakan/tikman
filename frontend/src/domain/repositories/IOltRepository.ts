import type {
  Olt,
  CreateOltDto,
  UpdateOltDto,
  OltStats,
  UnconfiguredOnu,
  OltVlan,
} from "../entities";

export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  getStats(id: string): Promise<OltStats>;
  getUnconfiguredOnus(id: string): Promise<UnconfiguredOnu[]>;
  getVlans(id: string): Promise<OltVlan[]>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
