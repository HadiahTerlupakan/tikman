import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  CreateRouterDto,
  MikrotikRouter,
  RouterBgpEvent,
  RouterBgpSession,
  RouterConnectionTestResult,
  RouterInterface,
  RouterTrafficPoint,
  TestRouterConnectionDto,
  TimeRange,
  UpdateRouterDto,
} from "@/domain/entities";

export class MikrotikRouterRepository {
  async list(): Promise<MikrotikRouter[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTERS)).data;
  }

  async create(data: CreateRouterDto): Promise<MikrotikRouter> {
    return (await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTERS, data)).data;
  }

  async update(id: string, data: UpdateRouterDto): Promise<MikrotikRouter> {
    return (await apiClient.put(API_ENDPOINTS.MIKROTIK_ROUTER_BY_ID(id), data))
      .data;
  }

  async remove(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MIKROTIK_ROUTER_BY_ID(id));
  }

  async testConnection(
    data: TestRouterConnectionDto,
  ): Promise<RouterConnectionTestResult> {
    return (await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_TEST, data))
      .data;
  }

  async refresh(id: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_REFRESH(id));
  }

  async trustCertificate(
    id: string,
    fingerprint: string,
  ): Promise<MikrotikRouter> {
    return (
      await apiClient.post(API_ENDPOINTS.MIKROTIK_ROUTER_TRUST(id), {
        fingerprint,
      })
    ).data;
  }

  async interfaces(id: string): Promise<RouterInterface[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_INTERFACES(id)))
      .data;
  }

  async sessions(id: string): Promise<RouterBgpSession[]> {
    return (await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_SESSIONS(id)))
      .data;
  }

  async events(id: string, range: TimeRange): Promise<RouterBgpEvent[]> {
    return (
      await apiClient.get(API_ENDPOINTS.MIKROTIK_ROUTER_EVENTS(id), {
        params: range,
      })
    ).data;
  }

  async traffic(
    id: string,
    interfaceId: string,
    range: TimeRange,
  ): Promise<RouterTrafficPoint[]> {
    return (
      await apiClient.get(
        API_ENDPOINTS.MIKROTIK_ROUTER_TRAFFIC(id, interfaceId),
        {
          params: range,
        },
      )
    ).data;
  }
}
