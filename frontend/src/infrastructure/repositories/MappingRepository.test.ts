import { beforeEach, describe, expect, it, vi } from "vitest";
import type { MappingEdge } from "@/domain/entities";
import { MappingRepository } from "./MappingRepository";

vi.mock("../http/apiClient", () => ({
  apiClient: { get: vi.fn(), post: vi.fn(), put: vi.fn(), delete: vi.fn() },
}));

const { apiClient } = await import("../http/apiClient");

describe("MappingRepository", () => {
  beforeEach(() => vi.clearAllMocks());

  it("unwraps the node list from its envelope", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({
      data: { data: [{ nodeId: "ODP-01", type: "odp", name: "Satu" }] },
    } as never);

    const nodes = await new MappingRepository().listNodes();

    expect(apiClient.get).toHaveBeenCalledWith("/api/v1/mapping/nodes");
    expect(nodes).toHaveLength(1);
    expect(nodes[0].nodeId).toBe("ODP-01");
  });

  it("addresses a node by its node id, not its uuid", async () => {
    vi.mocked(apiClient.delete).mockResolvedValue({ data: {} } as never);

    await new MappingRepository().deleteNode("ODP-01");

    expect(apiClient.delete).toHaveBeenCalledWith(
      "/api/v1/mapping/nodes/ODP-01",
    );
  });

  it("returns an empty list rather than undefined when there is no map yet", async () => {
    vi.mocked(apiClient.get).mockResolvedValue({ data: {} } as never);

    await expect(new MappingRepository().listEdges()).resolves.toEqual([]);

    expect(apiClient.get).toHaveBeenCalledWith("/api/v1/mapping/edges");
  });

  it("addresses a cable by its edge id when updating", async () => {
    const edge: MappingEdge = {
      edgeId: "ODC-01--ODP-01",
      source: "ODC-01",
      target: "ODP-01",
      fiberType: "distribution",
      distance: 120,
      waypoints: [],
      notes: "",
    };
    vi.mocked(apiClient.put).mockResolvedValue({
      data: { data: edge },
    } as never);

    await new MappingRepository().updateEdge("ODC-01--ODP-01", edge);

    expect(apiClient.put).toHaveBeenCalledWith(
      "/api/v1/mapping/edges/ODC-01--ODP-01",
      edge,
    );
  });
});
