import type {
  Olt,
  CreateOltDto,
  UpdateOltDto,
  OltStats,
  UnconfiguredOnu,
  OltVlan,
  OltSystemSnapshot,
  AggregateTrafficPoint,
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
  refreshSystem(id: string): Promise<OltSystemSnapshot>;
  getAggregateTraffic(
    id: string,
    period: string,
    position?: { slot?: number; port?: number },
  ): Promise<AggregateTrafficPoint[]>;
  create(data: CreateOltDto): Promise<Olt>;
  update(id: string, data: UpdateOltDto): Promise<Olt>;
  delete(id: string): Promise<void>;
}
