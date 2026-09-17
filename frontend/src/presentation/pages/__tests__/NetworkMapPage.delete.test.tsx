import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ApiError } from "@/infrastructure/http";
import { NetworkMapPage } from "../NetworkMapPage";

// Split out of NetworkMapPage.test.tsx to keep either file under the
// project's line limit — deletion (with its own confirm-then-fail-then-
// message paths) is a big enough surface on its own. The mock setup below is
// deliberately a full copy of the other file's rather than a shared import:
// vi.mock factories are hoisted above this file's own imports, so a value
// imported from a sibling fixtures module is still in its temporal dead zone
// the first time such a factory runs.
const { nodes, edges } = vi.hoisted(() => ({
  nodes: [
    {
      nodeId: "ODC-01",
      type: "odc",
      name: "ODC Satu",
      latitude: -6.2,
      longitude: 106.8,
      capacity: 8,
    },
    {
      nodeId: "ODP-01",
      type: "odp",
      name: "ODP Satu",
      latitude: -6.21,
      longitude: 106.81,
      capacity: 8,
    },
  ],
  edges: [
    {
      edgeId: "ODC-01--ODP-01",
      source: "ODC-01",
      target: "ODP-01",
      fiberType: "distribution",
      distance: 120,
      waypoints: [],
      notes: "",
    },
  ],
}));

const createNodeMutateAsync = vi.hoisted(() => vi.fn());
const updateNodeMutateAsync = vi.hoisted(() => vi.fn());
const deleteNodeMutateAsync = vi.hoisted(() => vi.fn());
const createEdgeMutateAsync = vi.hoisted(() => vi.fn());
const updateEdgeMutateAsync = vi.hoisted(() => vi.fn());
const deleteEdgeMutateAsync = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useMappingNodes: () => ({ data: nodes, isLoading: false }),
  useMappingEdges: () => ({ data: edges, isLoading: false }),
  useCreateNode: () => ({
    mutateAsync: createNodeMutateAsync,
    isPending: false,
  }),
  useUpdateNode: () => ({
    mutateAsync: updateNodeMutateAsync,
    isPending: false,
  }),
  useDeleteNode: () => ({
    mutateAsync: deleteNodeMutateAsync,
    isPending: false,
  }),
  useCreateEdge: () => ({
    mutateAsync: createEdgeMutateAsync,
    isPending: false,
  }),
  useUpdateEdge: () => ({
    mutateAsync: updateEdgeMutateAsync,
    isPending: false,
  }),
  useDeleteEdge: () => ({
    mutateAsync: deleteEdgeMutateAsync,
    isPending: false,
  }),
  useGoogleMapsKey: () => ({
    key: "test-key",
    mapId: "test-map",
    isLoading: false,
  }),
}));

// This suite never traces a cable or opens the node form, so the canvas can
// stay an inert stub — no callbacks need capturing the way
// NetworkMapPage.test.tsx does for the drawing flow.
vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: () => <div data-testid="map-canvas" />,
}));

const messageSuccess = vi.hoisted(() => vi.fn());
const messageError = vi.hoisted(() => vi.fn());

vi.mock("antd", async () => {
  const antd = await vi.importActual<typeof import("antd")>("antd");
  return {
    ...antd,
    message: { success: messageSuccess, error: messageError },
  };
});

// Both delete tables gained a Popconfirm (see NodeList/EdgeList): a bare
// click now only opens it, and the mutation only fires once "Ya" is clicked.
async function confirmDelete(rowText: string) {
  const row = screen.getByText(rowText).closest("tr")!;
  await userEvent.click(within(row).getByRole("button", { name: "Hapus" }));
  await userEvent.click(await screen.findByRole("button", { name: "Ya" }));
}

describe("NetworkMapPage deletion", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // A misclick in a 20-row table used to remove a node outright, orphaning
  // every cable drawn to it with no undo. Popconfirm is what the rest of the
  // codebase (OltTable, SiteTable, UserTable, ConfigTemplateTable) already
  // uses for exactly this; a bare click must no longer be enough.
  it("deletes a cable from the Daftar view only after confirming, via useDeleteEdge not useDeleteNode", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    const edgeRow = screen.getByText("ODC-01--ODP-01").closest("tr")!;
    await userEvent.click(
      within(edgeRow).getByRole("button", { name: "Hapus" }),
    );
    expect(deleteEdgeMutateAsync).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(deleteEdgeMutateAsync).toHaveBeenCalledWith("ODC-01--ODP-01");
    expect(deleteNodeMutateAsync).not.toHaveBeenCalled();
  });

  // The mirror of the cable-deletion test above: a copy-paste of the wrong
  // hook between NodeList's and EdgeList's onDelete would pass everything
  // else and only show up here.
  it("deletes a node from the Daftar view only after confirming, via useDeleteNode not useDeleteEdge", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    const nodeRow = screen.getByText("ODC Satu").closest("tr")!;
    await userEvent.click(
      within(nodeRow).getByRole("button", { name: "Hapus" }),
    );
    expect(deleteNodeMutateAsync).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(deleteNodeMutateAsync).toHaveBeenCalledWith("ODC-01");
    expect(deleteEdgeMutateAsync).not.toHaveBeenCalled();
  });

  // DeleteNode now refuses with 409 NODE_IN_USE while an ONT still points at
  // the box, naming how many. Swallowing that would leave the row sitting
  // there with no explanation of why it did not disappear.
  it("names how many ONTs still hold a node before refusing to delete it", async () => {
    deleteNodeMutateAsync.mockRejectedValueOnce(
      new ApiError(
        409,
        "NODE_IN_USE",
        undefined,
        "node masih dipakai: 3 ONT masih terhubung ke ODP-01",
      ),
    );
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await confirmDelete("ODC Satu");

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith(
        "node masih dipakai: 3 ONT masih terhubung ke ODP-01",
      ),
    );
  });

  it("falls back to a generic message when a node fails to delete for another reason", async () => {
    deleteNodeMutateAsync.mockRejectedValueOnce(new Error("network down"));
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await confirmDelete("ODC Satu");

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith("Gagal menghapus node"),
    );
  });

  it("falls back to a generic message when a cable fails to delete", async () => {
    deleteEdgeMutateAsync.mockRejectedValueOnce(new Error("network down"));
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await confirmDelete("ODC-01--ODP-01");

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith("Gagal menghapus kabel"),
    );
  });
});
