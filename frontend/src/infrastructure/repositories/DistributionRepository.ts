import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import { MappingRepository } from "./MappingRepository";
import type {
  CreateOdcDto,
  CreateOdpDto,
  MappingNode,
  Odc,
  OdcFeed,
  Odp,
  RoutePoint,
} from "@/domain/entities";
import type { Ont } from "@/domain/entities";

// The plant's own /odps table was deleted with the rest of the fixed ODC/ODP
// package (Task 6); a distribution box is now a mapping_nodes row of type
// odp. Occupancy is not tracked on the node itself — the old table's
// usedPorts counted a join this API no longer has, and getting it right would
// mean either one query per box or a new aggregate endpoint, both out of
// reach here — so it reads as 0 until that is worth adding.
function toOdp(node: MappingNode): Odp {
  return {
    id: node.id ?? node.nodeId,
    code: node.nodeId,
    portCount: node.capacity,
    usedPorts: 0,
    latitude: node.latitude,
    longitude: node.longitude,
    address: "",
    notes: node.notes,
    routeMeters: 0,
  };
}

/**
 * DistributionRepository reaches the fibre plant: cabinets, the ports feeding
 * them, and the distribution boxes a subscriber's drop lands in.
 *
 * Query parameters are spelled the way the API reads them — snake_case — which
 * is the call site's job here: the request interceptor decamelizes the body and
 * never the query string.
 */
export class DistributionRepository {
  private readonly mapping = new MappingRepository();

  async listOdcs(): Promise<Odc[]> {
    const response = await apiClient.get(API_ENDPOINTS.ODCS);
    return response.data.data ?? [];
  }

  async createOdc(data: CreateOdcDto): Promise<Odc> {
    const response = await apiClient.post(API_ENDPOINTS.ODCS, data);
    return response.data.data;
  }

  async listOdcFeeds(): Promise<OdcFeed[]> {
    const response = await apiClient.get(API_ENDPOINTS.ODC_FEED_LIST);
    return response.data.data ?? [];
  }

  /** An empty path hands the cable back to the straight line the map draws. */
  async setOdpRoute(odpId: string, route: RoutePoint[]): Promise<void> {
    await apiClient.put(API_ENDPOINTS.ODP_ROUTE(odpId), { route });
  }

  async setOdcFeedRoute(feedId: string, route: RoutePoint[]): Promise<void> {
    await apiClient.put(API_ENDPOINTS.ODC_FEED_ROUTE(feedId), { route });
  }

  async listOdps(): Promise<Odp[]> {
    const nodes = await this.mapping.listNodes();
    return nodes.filter((node) => node.type === "odp").map(toOdp);
  }

  async createOdp(data: CreateOdpDto): Promise<Odp> {
    const response = await apiClient.post(API_ENDPOINTS.ODPS, data);
    return response.data.data;
  }

  /** The ONT list narrowed to one box, since /odps/:id/subscribers is gone. */
  async subscribersOn(odpId: string): Promise<Ont[]> {
    const response = await apiClient.get(API_ENDPOINTS.ONTS, {
      params: { odp_id: odpId },
    });
    return response.data.data ?? [];
  }

  async assignOnt(ontId: string, odpId: string, port: number): Promise<void> {
    await apiClient.put(API_ENDPOINTS.ONT_ODP(ontId), { odpId, port });
  }

  async unassignOnt(ontId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.ONT_ODP(ontId));
  }
}
