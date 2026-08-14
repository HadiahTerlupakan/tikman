import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { ISiteRepository } from "@/domain/repositories";
import type { Site, CreateSiteDto, UpdateSiteDto } from "@/domain/entities";

export class SiteRepository implements ISiteRepository {
  async getAll(): Promise<Site[]> {
    const response = await apiClient.get(API_ENDPOINTS.SITES);
    return response.data;
  }

  async getById(id: string): Promise<Site> {
    const response = await apiClient.get(API_ENDPOINTS.SITE_BY_ID(id));
    return response.data;
  }

  async create(data: CreateSiteDto): Promise<Site> {
    const response = await apiClient.post(API_ENDPOINTS.SITES, data);
    return response.data;
  }

  async update(id: string, data: UpdateSiteDto): Promise<Site> {
    const response = await apiClient.put(API_ENDPOINTS.SITE_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.SITE_BY_ID(id));
  }
}
