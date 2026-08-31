import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IOltRepository } from "@/domain/repositories";
import type {
  Olt,
  CreateOltDto,
  UpdateOltDto,
  OltStats,
  UnconfiguredOnu,
  OltVlan,
  OltSystemSnapshot,
  AggregateTrafficPoint,
} from "@/domain/entities";

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

  async getStats(id: string): Promise<OltStats> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_STATS(id));
    return response.data;
  }

  async getUnconfiguredOnus(id: string): Promise<UnconfiguredOnu[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.OLT_UNCONFIGURED_ONUS(id),
    );
    return response.data.data ?? [];
  }

  async getVlans(id: string): Promise<OltVlan[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_VLANS(id));
    return response.data.data ?? [];
  }

  async getTcontProfiles(id: string): Promise<string[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_TCONT_PROFILES(id));
    return response.data.data ?? [];
  }

  async getVlanProfiles(id: string): Promise<string[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_VLAN_PROFILES(id));
    return response.data.data ?? [];
  }

  async getSystem(id: string): Promise<OltSystemSnapshot> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_SYSTEM(id));
    return response.data.data;
  }

  async getAggregateTraffic(
    id: string,
    period: string,
    position?: { slot?: number; port?: number },
  ): Promise<AggregateTrafficPoint[]> {
    const params: Record<string, string> = { period };
    if (position?.slot !== undefined) params.slot = String(position.slot);
    if (position?.port !== undefined) params.port = String(position.port);
    const response = await apiClient.get(API_ENDPOINTS.OLT_TRAFFIC(id), {
      params,
    });
    return response.data.points ?? [];
  }

  async refreshSystem(id: string): Promise<OltSystemSnapshot> {
    const response = await apiClient.post(API_ENDPOINTS.OLT_SYSTEM_REFRESH(id));
    return response.data.data;
  }

  // Schedules the inventory pass rather than performing it: discovery takes
  // minutes on a populated chassis, so the worker runs it and the OLT page's
  // progress follows along.
  async discoverNow(id: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.OLT_DISCOVER_NOW(id));
  }

  async getOnuTypes(id: string): Promise<string[]> {
    const response = await apiClient.get(API_ENDPOINTS.OLT_ONU_TYPES(id));
    return response.data.data ?? [];
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
      data,
    );
    return response.data;
  }
}
