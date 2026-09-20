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

  // The KMZ is a zip, not JSON: it must be asked for as a blob, or the shared
  // response interceptor's camelizeKeys would silently replace it with `{}`.
  it("asks for the export as a blob", async () => {
    const blob = new Blob(["fake kmz"], {
      type: "application/vnd.google-earth.kmz",
    });
    vi.mocked(apiClient.get).mockResolvedValue({
      data: blob,
      headers: {},
    } as never);

    const result = await new MappingRepository().exportKmz();

    expect(apiClient.get).toHaveBeenCalledWith("/api/v1/mapping/export", {
      responseType: "blob",
    });
    expect(result.blob).toBe(blob);
    expect(result.warning).toBe("");
  });

  // The backend leaves the header off entirely on a clean export (not
  // present-but-empty) - a skipped cable's warning must still reach the
  // person downloading the file, not only the description inside it.
  it("carries the skipped-cable warning header through when present", async () => {
    const warning =
      "1 kabel dilewati karena salah satu ujungnya sudah dihapus dari peta";
    vi.mocked(apiClient.get).mockResolvedValue({
      data: new Blob(["fake kmz"]),
      headers: { "x-kmz-warning": warning },
    } as never);

    const result = await new MappingRepository().exportKmz();

    expect(result.warning).toBe(warning);
  });

  // A .kmz upload has to travel as multipart form data, not JSON - the same
  // reason CsRepository.sendMedia drops the Content-Type header rather than
  // leaving axios to JSON-encode the FormData and lose the file.
  it("uploads the chosen file as multipart form data for a preview", async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      data: { data: { nodes: [], edges: [], issues: [], totalPlacemarks: 0 } },
    } as never);
    const file = new File(["isi kmz"], "peta.kmz");

    await new MappingRepository().previewImport(file);

    expect(apiClient.post).toHaveBeenCalledWith(
      "/api/v1/mapping/import/preview",
      expect.any(FormData),
      { headers: { "Content-Type": false } },
    );
    const sentForm = vi.mocked(apiClient.post).mock.calls[0][1] as FormData;
    expect(sentForm.get("file")).toBe(file);
  });

  it("sends the confirmed rows as JSON when committing an import", async () => {
    vi.mocked(apiClient.post).mockResolvedValue({
      data: { data: { nodesCreated: 1, edgesCreated: 0 } },
    } as never);
    const node = {
      row: 1,
      nodeId: "ODP-01",
      type: "odp" as const,
      name: "ODP Satu",
      latitude: -6.2,
      longitude: 106.8,
      capacity: 0,
      splitter: "",
      pppoe: "",
      serialNumber: "",
      notes: "",
      reason: "",
      conflict: false,
      conflictReason: "",
      blocked: false,
      blockedReason: "",
      include: true,
    };

    const result = await new MappingRepository().commitImport([node], []);

    expect(apiClient.post).toHaveBeenCalledWith(
      "/api/v1/mapping/import/commit",
      { nodes: [node], edges: [] },
    );
    expect(result.nodesCreated).toBe(1);
  });
});
