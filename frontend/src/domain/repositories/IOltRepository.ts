import type {
  Olt,
  CreateOltDto,
  UpdateOltDto,
  OltStats,
  UnconfiguredOnu,
} from "../entities";

export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  getStats(id: string): Promise<OltStats>;
  getUnconfiguredOnus(id: string): Promise<UnconfiguredOnu[]>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
