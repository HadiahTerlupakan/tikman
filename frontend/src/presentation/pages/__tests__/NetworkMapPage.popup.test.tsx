import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingEdge, MappingNode, Waypoint } from "@/domain/entities";
import { ApiError } from "@/infrastructure/http";
import { NetworkMapPage } from "../NetworkMapPage";

// Split out of NetworkMapPage.test.tsx for the same reason delete and redraw
// already are: the popup is its own surface (which of two mutually-exclusive
// selections is open, and five action callbacks that must reach the exact
// same handlers the Daftar view uses) big enough to keep either file under
// the project's line limit. The mock setup is a full copy rather than a
// shared import — vi.mock factories are hoisted above this file's own
// imports, so a value imported from a sibling fixtures module is still in
// its temporal dead zone the first time such a factory runs.
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

interface MockCanvasProps {
  fromNodeId?: string;
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
  onEdgeClick: (edgeId: string) => void;
  popup: {
    selectedNode?: MappingNode;
    selectedEdge?: MappingEdge;
    onClose: () => void;
    actions: {
      onEditNode: (node: MappingNode) => void;
      // Typed loosely (not void) so a test can await the underlying
      // removeNode/removeEdge promise directly, rather than relying on
      // waitFor to notice its rejection handling completed — see the
      // NODE_IN_USE test below.
      onDeleteNode: (nodeId: string) => unknown;
      onEditEdge: (edge: MappingEdge) => void;
      onRedrawEdge: (edge: MappingEdge) => void;
      onDeleteEdge: (edgeId: string) => unknown;
    };
  };
}

// Capturing the callbacks and the selection props (rather than rendering an
// inert div) is what lets a test drive "click a node" and inspect what the
// page decided to show, without a real map — the same technique the delete
// and redraw suites use for this exact mock.
let canvasProps: MockCanvasProps;

vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: (props: MockCanvasProps) => {
    canvasProps = props;
    return <div data-testid="map-canvas" />;
  },
}));

const messageError = vi.hoisted(() => vi.fn());

vi.mock("antd", async () => {
  const antd = await vi.importActual<typeof import("antd")>("antd");
  return { ...antd, message: { success: vi.fn(), error: messageError } };
});

describe("NetworkMapPage popups", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("opens a node's popup on a plain click, outside any cable tracing", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.onNodeClick("ODP-01"));

    expect(canvasProps.popup.selectedNode).toEqual(
      expect.objectContaining({ nodeId: "ODP-01", name: "ODP Satu" }),
    );
  });

  // The one thing this feature must not break: a node click while a fresh
  // cable is being traced has to keep starting/extending that cable, and
  // must not also open a popup alongside it.
  it("still traces instead of opening a popup when a node is clicked while a cable is being drawn", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));

    expect(canvasProps.fromNodeId).toBe("ODC-01");
    expect(canvasProps.popup.selectedNode).toBeUndefined();
  });

  // The redraw guard (ignore node taps so an endpoint is never reassigned)
  // and the new popup behaviour must not collide: redrawing still opens no
  // popup either.
  it("opens no popup for a node click while a cable's route is being redrawn", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await userEvent.click(
      screen.getAllByRole("button", { name: "Gambar ulang" })[0],
    );
    act(() => canvasProps.onNodeClick("ODP-01"));

    expect(canvasProps.popup.selectedNode).toBeUndefined();
  });

  it("opens a cable's popup on a plain click, outside any cable tracing", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.onEdgeClick("ODC-01--ODP-01"));

    expect(canvasProps.popup.selectedEdge).toEqual(
      expect.objectContaining({ edgeId: "ODC-01--ODP-01" }),
    );
  });

  it("does not open a cable's popup while a cable is being drawn", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onEdgeClick("ODC-01--ODP-01"));

    expect(canvasProps.popup.selectedEdge).toBeUndefined();
  });

  it("closes whichever popup is open", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.onNodeClick("ODP-01"));
    act(() => canvasProps.popup.onClose());

    expect(canvasProps.popup.selectedNode).toBeUndefined();
  });

  it("edits the popup's node through the same form NodeList's Ubah uses", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.popup.actions.onEditNode(nodes[0] as MappingNode));

    expect(screen.getByText("Ubah node")).toBeInTheDocument();
  });

  it("deletes the popup's node via useDeleteNode, not useDeleteEdge", async () => {
    render(<NetworkMapPage />);

    await act(async () => {
      await canvasProps.popup.actions.onDeleteNode("ODP-01");
    });

    expect(deleteNodeMutateAsync).toHaveBeenCalledWith("ODP-01");
    expect(deleteEdgeMutateAsync).not.toHaveBeenCalled();
  });

  // The exact requirement: NODE_IN_USE must surface as itself from the map
  // popup too, not just from the Daftar table's Hapus.
  it("names how many ONTs still hold a node deleted from its map popup", async () => {
    deleteNodeMutateAsync.mockRejectedValueOnce(
      new ApiError(
        409,
        "NODE_IN_USE",
        undefined,
        "node masih dipakai: 2 ONT masih terhubung ke ODP-01",
      ),
    );
    render(<NetworkMapPage />);

    await act(async () => {
      await canvasProps.popup.actions.onDeleteNode("ODP-01");
    });

    expect(messageError).toHaveBeenCalledWith(
      "node masih dipakai: 2 ONT masih terhubung ke ODP-01",
    );
  });

  it("edits the popup's cable through the same form EdgeList's Ubah uses", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.popup.actions.onEditEdge(edges[0] as MappingEdge));

    expect(screen.getByText("Ubah kabel")).toBeInTheDocument();
  });

  it("deletes the popup's cable via useDeleteEdge, not useDeleteNode", async () => {
    render(<NetworkMapPage />);

    await act(async () => {
      await canvasProps.popup.actions.onDeleteEdge("ODC-01--ODP-01");
    });

    expect(deleteEdgeMutateAsync).toHaveBeenCalledWith("ODC-01--ODP-01");
    expect(deleteNodeMutateAsync).not.toHaveBeenCalled();
  });

  // Gambar ulang from the popup must be the exact same entry point as
  // EdgeList's own button: armed at the cable's existing source, on the map.
  it("starts the same redraw from the popup that EdgeList's Gambar ulang starts", () => {
    render(<NetworkMapPage />);

    act(() => canvasProps.popup.actions.onRedrawEdge(edges[0] as MappingEdge));

    expect(canvasProps.fromNodeId).toBe("ODC-01");
  });
});
