import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { CreateOdcDto, CreateOdpDto, Odc, Odp } from "@/domain/entities";
import type { Ont } from "@/domain/entities";

/**
 * DistributionRepository reaches the fibre plant: cabinets, the ports feeding
 * them, and the distribution boxes a subscriber's drop lands in.
 *
 * Query parameters are spelled the way the API reads them — snake_case — which
 * is the call site's job here: the request interceptor decamelizes the body and
 * never the query string.
 */
export class DistributionRepository {
  async listOdcs(): Promise<Odc[]> {
    const response = await apiClient.get(API_ENDPOINTS.ODCS);
    return response.data.data ?? [];
  }

  async createOdc(data: CreateOdcDto): Promise<Odc> {
    const response = await apiClient.post(API_ENDPOINTS.ODCS, data);
    return response.data.data;
  }

  async listOdps(): Promise<Odp[]> {
    const response = await apiClient.get(API_ENDPOINTS.ODPS);
    return response.data.data ?? [];
  }

  async createOdp(data: CreateOdpDto): Promise<Odp> {
    const response = await apiClient.post(API_ENDPOINTS.ODPS, data);
    return response.data.data;
  }

  async subscribersOn(odpId: string): Promise<Ont[]> {
    const response = await apiClient.get(API_ENDPOINTS.ODP_SUBSCRIBERS(odpId));
    return response.data.data ?? [];
  }

  async assignOnt(ontId: string, odpId: string, port: number): Promise<void> {
    await apiClient.put(API_ENDPOINTS.ONT_ODP(ontId), { odpId, port });
  }

  async unassignOnt(ontId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.ONT_ODP(ontId));
  }
}
