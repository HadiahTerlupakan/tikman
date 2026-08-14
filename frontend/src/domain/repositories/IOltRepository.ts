import type { Olt, CreateOltDto, UpdateOltDto } from "../entities";

export interface IOltRepository {
  getAll(): Promise<Olt[]>;
  getBySite(siteId: string): Promise<Olt[]>;
  getById(id: string): Promise<Olt>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
