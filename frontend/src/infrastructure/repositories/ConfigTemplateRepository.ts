import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IConfigTemplateRepository } from "@/domain/repositories/IConfigTemplateRepository";
import type {
  ConfigTemplate,
  CreateConfigTemplateDto,
  UpdateConfigTemplateDto,
} from "@/domain/entities/ConfigTemplate";

export class ConfigTemplateRepository implements IConfigTemplateRepository {
  async getAll(): Promise<ConfigTemplate[]> {
    const response = await apiClient.get<{ data: ConfigTemplate[] }>(
      API_ENDPOINTS.CONFIG_TEMPLATES,
    );
    return response.data.data;
  }

  async getById(id: string): Promise<ConfigTemplate> {
    const response = await apiClient.get<{ data: ConfigTemplate }>(
      API_ENDPOINTS.CONFIG_TEMPLATE_BY_ID(id),
    );
    return response.data.data;
  }

  async create(data: CreateConfigTemplateDto): Promise<ConfigTemplate> {
    const response = await apiClient.post<{ data: ConfigTemplate }>(
      API_ENDPOINTS.CONFIG_TEMPLATES,
      data,
    );
    return response.data.data;
  }

  async update(
    id: string,
    data: UpdateConfigTemplateDto,
  ): Promise<ConfigTemplate> {
    const response = await apiClient.put<{ data: ConfigTemplate }>(
      API_ENDPOINTS.CONFIG_TEMPLATE_BY_ID(id),
      data,
    );
    return response.data.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.CONFIG_TEMPLATE_BY_ID(id));
  }
}
