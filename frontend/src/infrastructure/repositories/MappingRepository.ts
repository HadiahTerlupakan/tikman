import type { MappingEdge, MappingNode } from "@/domain/entities";
import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";

export class MappingRepository {
  async listNodes(): Promise<MappingNode[]> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_NODES);
    return res.data.data ?? [];
  }

  async createNode(node: MappingNode): Promise<MappingNode> {
    const res = await apiClient.post(API_ENDPOINTS.MAPPING_NODES, node);
    return res.data.data;
  }

  async updateNode(nodeId: string, node: MappingNode): Promise<MappingNode> {
    const res = await apiClient.put(API_ENDPOINTS.MAPPING_NODE(nodeId), node);
    return res.data.data;
  }

  async deleteNode(nodeId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MAPPING_NODE(nodeId));
  }

  async listEdges(): Promise<MappingEdge[]> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_EDGES);
    return res.data.data ?? [];
  }

  async createEdge(edge: MappingEdge): Promise<MappingEdge> {
    const res = await apiClient.post(API_ENDPOINTS.MAPPING_EDGES, edge);
    return res.data.data;
  }

  async updateEdge(edgeId: string, edge: MappingEdge): Promise<MappingEdge> {
    const res = await apiClient.put(API_ENDPOINTS.MAPPING_EDGE(edgeId), edge);
    return res.data.data;
  }

  async deleteEdge(edgeId: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.MAPPING_EDGE(edgeId));
  }

  // A blob, not JSON: the backend answers a .kmz archive, and the shared
  // response interceptor knows to leave a blob response untouched.
  async exportKmz(): Promise<Blob> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_EXPORT, {
      responseType: "blob",
    });
    return res.data;
  }
}
