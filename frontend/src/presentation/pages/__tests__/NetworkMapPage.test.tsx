import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Waypoint } from "@/domain/entities";
import { NetworkMapPage } from "../NetworkMapPage";

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
      waypoints: [],
      notes: "",
    },
  ],
}));

const createNodeMutateAsync = vi.hoisted(() => vi.fn());
const updateNodeMutateAsync = vi.hoisted(() => vi.fn());
const deleteNodeMutateAsync = vi.hoisted(() => vi.fn());
const createEdgeMutateAsync = vi.hoisted(() => vi.fn());
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
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
}

// The canvas needs a Google Maps key and a live network; the page's own job is
// counting and wiring. Capturing its callbacks (rather than rendering an inert
// div) is what lets a test drive "click a node, drop a point" without a real
// map — the same technique MapCanvas.test.tsx uses for the Google Maps mock.
let canvasProps: MockCanvasProps;

vi.mock("../../components/netmap/MapCanvas", () => ({
  MapCanvas: (props: MockCanvasProps) => {
    canvasProps = props;
    return <div data-testid="map-canvas" />;
  },
}));

describe("NetworkMapPage", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("counts what is on the map, by kind", () => {
    render(<NetworkMapPage />);

    expect(screen.getByTestId("count-odc")).toHaveTextContent("1");
    expect(screen.getByTestId("count-odp")).toHaveTextContent("2");
    expect(screen.getByTestId("count-ont")).toHaveTextContent("0");
  });

  it("offers the button that starts a cable", () => {
    render(<NetworkMapPage />);

    expect(
      screen.getByRole("button", { name: /Tarik kabel/ }),
    ).toBeInTheDocument();
  });

  // The spec: "Daftar menampilkan node dan kabel dalam tabel" — a list missing
  // either table silently drops half of what it promises to manage.
  it("shows both the node table and the cable table in the list view", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));

    expect(screen.getByText("ODC Satu")).toBeInTheDocument();
    expect(screen.getByText("ODC-01--ODP-01")).toBeInTheDocument();
  });

  it("edits a node through Ubah, saving via useUpdateNode rather than creating a new one", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await userEvent.click(screen.getAllByRole("button", { name: "Ubah" })[0]);
    expect(screen.getByText("Ubah node")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(updateNodeMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ nodeId: "ODC-01" }),
    );
    expect(createNodeMutateAsync).not.toHaveBeenCalled();
  });

  it("deletes a cable from the Daftar view via useDeleteEdge, not useDeleteNode", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    const edgeRow = screen.getByText("ODC-01--ODP-01").closest("tr")!;
    await userEvent.click(
      within(edgeRow).getByRole("button", { name: "Hapus" }),
    );

    expect(deleteEdgeMutateAsync).toHaveBeenCalledWith("ODC-01--ODP-01");
    expect(deleteNodeMutateAsync).not.toHaveBeenCalled();
  });

  // The bug this task exists to fix: an earlier sample saved every cable as
  // "distribution" regardless of what the operator picked.
  it("saves the cable with the fiber type chosen in the modal, not the default", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-02"));

    await userEvent.click(screen.getByRole("combobox"));
    await userEvent.click(await screen.findByTitle("ODP ke ODP"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(createEdgeMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        source: "ODC-01",
        target: "ODP-02",
        fiberType: "odp_to_odp",
      }),
    );
    expect(createEdgeMutateAsync).not.toHaveBeenCalledWith(
      expect.objectContaining({ fiberType: "distribution" }),
    );
  });

  // Spec: "Satu klik yang salah dibatalkan satu titik, bukan seluruh jalur."
  // A technician tracing a kilometre of fibre along a road will misclick, and
  // losing the whole path to one tap is what makes a tool get abandoned.
  it("keeps only the first two points after tracing three and undoing once", async () => {
    render(<NetworkMapPage />);

    expect(
      screen.queryByRole("button", { name: "Batal titik" }),
    ).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onDrop({ lat: -6.001, lng: 106.001 }));
    act(() => canvasProps.onDrop({ lat: -6.002, lng: 106.002 }));
    act(() => canvasProps.onDrop({ lat: -6.003, lng: 106.003 }));

    await userEvent.click(screen.getByRole("button", { name: "Batal titik" }));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(createEdgeMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({
        source: "ODC-01",
        target: "ODP-01",
        waypoints: [
          { lat: -6.001, lng: 106.001 },
          { lat: -6.002, lng: 106.002 },
        ],
      }),
    );
  });
});
