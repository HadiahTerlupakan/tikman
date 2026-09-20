import type {
  ImportedEdge,
  ImportedNode,
  ImportPreview,
  ImportResult,
  MappingEdge,
  MappingNode,
} from "@/domain/entities";
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
  // response interceptor knows to leave a blob response untouched. The
  // warning header is absent entirely on a clean export (not present but
  // empty), hence the "" default rather than leaving it undefined.
  async exportKmz(): Promise<{ blob: Blob; warning: string }> {
    const res = await apiClient.get(API_ENDPOINTS.MAPPING_EXPORT, {
      responseType: "blob",
    });
    return { blob: res.data, warning: res.headers["x-kmz-warning"] ?? "" };
  }

  // Multipart, not JSON, the same reason CsRepository.sendMedia drops the
  // Content-Type header: leaving the client's default in place would make
  // axios JSON-encode the FormData instead of sending the file itself.
  async previewImport(file: File): Promise<ImportPreview> {
    const form = new FormData();
    form.append("file", file);
    const res = await apiClient.post(
      API_ENDPOINTS.MAPPING_IMPORT_PREVIEW,
      form,
      { headers: { "Content-Type": false } },
    );
    return res.data.data;
  }

  // Sends back exactly the preview's own row shape, corrected in place -
  // there is no separate "commit" shape to translate into.
  async commitImport(
    nodes: ImportedNode[],
    edges: ImportedEdge[],
  ): Promise<ImportResult> {
    const res = await apiClient.post(API_ENDPOINTS.MAPPING_IMPORT_COMMIT, {
      nodes,
      edges,
    });
    return res.data.data;
  }
}
