import type { Site, CreateSiteDto, UpdateSiteDto } from '../entities';

export interface ISiteRepository {
  getAll(): Promise<Site[]>;
  getById(id: string): Promise<Site>;
  create(data: CreateSiteDto): Promise<Site>;
  update(id: string, data: UpdateSiteDto): Promise<Site>;
  delete(id: string): Promise<void>;
}
