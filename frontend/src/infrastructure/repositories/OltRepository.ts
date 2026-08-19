import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IOltRepository } from "@/domain/repositories";
import type { Olt, CreateOltDto, UpdateOltDto } from "@/domain/entities";

export class OltRepository implements IOltRepository {
  async getAll(): Promise<Olt[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLTS);
    return response.data;
  }

  async getBySite(siteId: string): Promise<Olt[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLTS, {
      params: { site_id: siteId },
    });
    return response.data;
  }

  async getById(id: string): Promise<Olt> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_BY_ID(id));
    return response.data;
  }

  async create(data: CreateOltDto): Promise<Olt> {
    const response = await apiClient.post(API_ENDPOINTS.OLTS, data);
    return response.data;
  }

  async update(id: string, data: UpdateOltDto): Promise<Olt> {
    const response = await apiClient.put(API_ENDPOINTS.OLT_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.OLT_BY_ID(id));
  }

  async testConnection(data: {
    ipAddress: string;
    username: string;
    password: string;
    preferredProtocol: string;
    sshPort?: number;
    telnetPort?: number;
    snmpPort?: number;
    snmpCommunity?: string;
    rack?: number;
    shelf?: number;
    slot?: number;
  }): Promise<{
    success: boolean;
    passedTests: string[];
    failedTest?: string;
    failedReason?: string;
  }> {
    const response = await apiClient.post(
      API_ENDPOINTS.TEST_OLT_CONNECTION,
      data
    );
    return response.data;
  }
}
