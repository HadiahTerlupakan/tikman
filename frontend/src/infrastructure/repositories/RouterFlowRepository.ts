import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  FlowDestinations,
  FlowFilter,
  FlowPeering,
  FlowPopRow,
  FlowStatus,
  FlowSummary,
  FlowTopIp,
  TimeRange,
} from "@/domain/entities";

export class RouterFlowRepository {
  private async read<T>(id: string, read: string, params: object = {}) {
    const response = await apiClient.get<T>(
      API_ENDPOINTS.MIKROTIK_ROUTER_FLOWS(id, read),
      { params },
    );
    return response.data;
  }

  destinations(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowDestinations> {
    return this.read(id, "destinations", { ...range, ...filter });
  }

  summary(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowSummary> {
    return this.read(id, "summary", { ...range, ...filter });
  }

  pops(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowPopRow[]> {
    return this.read(id, "pops", { ...range, ...filter });
  }

  topIps(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
  ): Promise<FlowTopIp[]> {
    return this.read(id, "top-ips", { ...range, ...filter });
  }

  peering(
    id: string,
    range: TimeRange,
    filter: FlowFilter,
    asn: number,
  ): Promise<FlowPeering> {
    return this.read(id, "peering", { ...range, ...filter, asn });
  }

  status(id: string): Promise<FlowStatus> {
    return this.read(id, "status");
  }
}
