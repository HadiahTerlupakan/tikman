import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { RouteLookup } from "@/domain/entities";

export class RouterPathRepository {
  async lookup(id: string, target: string): Promise<RouteLookup> {
    const response = await apiClient.get<RouteLookup>(
      API_ENDPOINTS.MIKROTIK_ROUTER_BGP(id, "lookup"),
      { params: { target } },
    );
    return response.data;
  }

  /** Names keyed by AS number; an AS the dataset lacks is absent. */
  async asNames(id: string, asns: number[]): Promise<Record<string, string>> {
    const response = await apiClient.get<Record<string, string>>(
      API_ENDPOINTS.MIKROTIK_ROUTER_BGP(id, "as-names"),
      { params: { asn: asns.join(",") } },
    );
    return response.data;
  }
}
