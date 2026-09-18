import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingNode, Waypoint } from "@/domain/entities";
import {
  edgePath,
  metersAlong,
} from "@/presentation/components/netmap/cableMath";
import { NetworkMapPage } from "../NetworkMapPage";

// Split out of NetworkMapPage.test.tsx for the same reason
// NetworkMapPage.delete.test.tsx was: redrawing is its own surface (arming
// the canvas at a fixed source, ignoring node taps, recomputing on finish)
// big enough to keep either file under the project's line limit. The mock
// setup below is a full copy rather than a shared import — vi.mock factories
// are hoisted above this file's own imports, so a value imported from a
// sibling fixtures module is still in its temporal dead zone the first time
// such a factory runs.
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
    {
      nodeId: "ODP-02",
      type: "odp",
      name: "ODP Dua",
      latitude: -6.22,
      longitude: 106.82,
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
      waypoints: [{ lat: -6.205, lng: 106.805 }],
      notes: "Kabel lama",
    },
  ],
}));

const createEdgeMutateAsync = vi.hoisted(() => vi.fn());
const updateEdgeMutateAsync = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useMappingNodes: () => ({ data: nodes, isLoading: false }),
  useMappingEdges: () => ({ data: edges, isLoading: false }),
  useCreateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useUpdateNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useDeleteNode: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCreateEdge: () => ({
    mutateAsync: createEdgeMutateAsync,
    isPending: false,
  }),
  useUpdateEdge: () => ({
    mutateAsync: updateEdgeMutateAsync,
    isPending: false,
  }),
  useDeleteEdge: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useGoogleMapsKey: () => ({
    key: "test-key",
    mapId: "test-map",
    isLoading: false,
  }),
}));

interface MockCanvasProps {
  draft: Waypoint[];
  fromNodeId?: string;
  redrawingEdgeId?: string;
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
}

// Capturing the callbacks (rather than rendering an inert div) is what lets a
// test drive "tap a corner, finish" without a real map — the same technique
// NetworkMapPage.test.tsx uses for the fresh-draw flow.
let canvasProps: MockCanvasProps;

vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: (props: MockCanvasProps) => {
    canvasProps = props;
    return <div data-testid="map-canvas" />;
  },
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

// Independently recomputes what a correct redraw must save, the same way
// cable creation does (edgePath + metersAlong) — a second, separate
// implementation here would let a mutant that miscomputes distance pass
// unnoticed, exactly the bug that shipped when drawing and measuring drifted
// apart the first time.
function expectedDistance(
  source: string,
  target: string,
  waypoints: Waypoint[],
) {
  const nodesById = new Map((nodes as MappingNode[]).map((n) => [n.nodeId, n]));
  return Math.round(
    metersAlong(edgePath({ source, target, waypoints }, nodesById)!),
  );
}

async function startRedraw() {
  render(<NetworkMapPage />);
  await userEvent.click(screen.getByText("Daftar"));
  const edgeRow = screen.getByText("ODC-01--ODP-01").closest("tr")!;
  await userEvent.click(
    within(edgeRow).getByRole("button", { name: "Gambar ulang" }),
  );
}

describe("NetworkMapPage redraw", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("switches to the map, armed at the cable's existing source, and away from the list", async () => {
    await startRedraw();

    expect(screen.getByTestId("map-canvas")).toBeInTheDocument();
    expect(screen.queryByText("ODC-01--ODP-01")).not.toBeInTheDocument();
    expect(canvasProps.fromNodeId).toBe("ODC-01");
    expect(canvasProps.redrawingEdgeId).toBe("ODC-01--ODP-01");
  });

  it("offers Batal titik and Selesai while redrawing, the same undo gesture as a fresh cable", async () => {
    await startRedraw();

    expect(
      screen.getByRole("button", { name: "Batal titik" }),
    ).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Selesai" })).toBeInTheDocument();
  });

  // The single strongest proof available: a full equality on the saved
  // payload catches a mutant touching any one field — the route not
  // replaced, the length left stale or computed from the old path, an
  // endpoint swapped for a tapped node, or the fiber type/notes dropped —
  // where several partial assertions could each miss what the others cover.
  it("saves the retraced route with a recomputed length, unchanged endpoints, and preserved fiber type and notes", async () => {
    await startRedraw();

    const newWaypoints = [{ lat: -6.19, lng: 106.79 }];
    act(() => canvasProps.onDrop(newWaypoints[0]));
    await userEvent.click(screen.getByRole("button", { name: "Selesai" }));

    const distance = expectedDistance("ODC-01", "ODP-01", newWaypoints);
    // Sanity check on the fixture itself: if this new corner happened to
    // recompute to the same 120m stored before the redraw, the test below
    // could pass even with the old distance saved unchanged.
    expect(distance).not.toBe(120);

    expect(updateEdgeMutateAsync).toHaveBeenCalledWith({
      edgeId: "ODC-01--ODP-01",
      edge: {
        edgeId: "ODC-01--ODP-01",
        source: "ODC-01",
        target: "ODP-01",
        fiberType: "distribution",
        notes: "Kabel lama",
        waypoints: newWaypoints,
        distance,
      },
    });
  });

  it("writes nothing when a redraw is cancelled", async () => {
    await startRedraw();

    act(() => canvasProps.onDrop({ lat: -6.19, lng: 106.79 }));
    await userEvent.click(screen.getByRole("button", { name: "Batal" }));

    expect(updateEdgeMutateAsync).not.toHaveBeenCalled();
    expect(
      screen.queryByRole("button", { name: "Selesai" }),
    ).not.toBeInTheDocument();
  });

  // Tapping a node is how a fresh cable finishes and picks its target, but a
  // redraw's target is already fixed. Treating this tap as "finish, and
  // repoint the cable at whatever was tapped" is the one thing this feature
  // must not do — endpoint reassignment is a separate decision, not made here.
  it("ignores a node tap while redrawing, rather than reassigning the endpoint", async () => {
    await startRedraw();

    act(() => canvasProps.onNodeClick("ODP-02"));

    expect(updateEdgeMutateAsync).not.toHaveBeenCalled();
    expect(createEdgeMutateAsync).not.toHaveBeenCalled();
    expect(screen.queryByText("Jenis kabel")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Selesai" })).toBeInTheDocument();
  });

  it("shows the underlying error when the redrawn cable fails to save", async () => {
    updateEdgeMutateAsync.mockRejectedValueOnce(new Error("network down"));
    await startRedraw();

    act(() => canvasProps.onDrop({ lat: -6.19, lng: 106.79 }));
    await userEvent.click(screen.getByRole("button", { name: "Selesai" }));

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith("network down"),
    );
    // Not just a toast: the redraw itself has to survive the rejection, or
    // the corners just traced are gone with nothing on screen saying so.
    // Selesai only renders while cable.redrawing is set, so its continued
    // presence is what proves the redraw was not silently abandoned. Before
    // the fix for the guard-falls-open bug, finish() cleared `redrawing`
    // unconditionally, so this assertion alone would already have failed
    // here — an error toast was never proof that anything else was intact.
    expect(screen.getByRole("button", { name: "Selesai" })).toBeInTheDocument();
  });

  // The critical failure mode: finishRedraw used to call cable.finish()
  // unconditionally before the mutation even started, clearing `redrawing`
  // (and `from`) regardless of whether the save succeeded. After a
  // rejection, `placing` was still "cable" while `redrawing` was already
  // undefined — exactly the condition that opens nodeTapped's guard
  // (`placing !== "cable" || cable.redrawing`). The next two node taps then
  // fell into the fresh-cable branch and silently started a cable nobody
  // asked for.
  it("does not let node taps after a failed save start a brand-new cable", async () => {
    updateEdgeMutateAsync.mockRejectedValueOnce(new Error("network down"));
    await startRedraw();

    act(() => canvasProps.onDrop({ lat: -6.19, lng: 106.79 }));
    await userEvent.click(screen.getByRole("button", { name: "Selesai" }));
    await waitFor(() => expect(messageError).toHaveBeenCalled());

    // Retapping two nodes, as if retrying out of habit — must not be
    // reinterpreted as "start tracing a fresh cable from scratch".
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-02"));

    expect(screen.queryByText("Jenis kabel")).not.toBeInTheDocument();
    expect(createEdgeMutateAsync).not.toHaveBeenCalled();
  });
});
