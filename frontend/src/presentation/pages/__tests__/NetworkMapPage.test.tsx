import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { Waypoint } from "@/domain/entities";
import { ApiError } from "@/infrastructure/http";
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
const updateEdgeMutateAsync = vi.hoisted(() => vi.fn());
const deleteEdgeMutateAsync = vi.hoisted(() => vi.fn());

vi.mock("@/application/hooks", () => ({
  useMappingNodes: () => ({ data: nodes, isLoading: false, refetch: vi.fn() }),
  useMappingEdges: () => ({ data: edges, isLoading: false }),
  // This suite never places an OLT; an empty fleet keeps that button a
  // no-op (see NetworkMapPage.oltPlacement.test.tsx for the real flow).
  useOlts: () => ({ data: [], isLoading: false }),
  useUpdateOlt: () => ({ mutateAsync: vi.fn(), isPending: false }),
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
  // MapToolbar's own export button; its download mechanics are covered by
  // MapToolbar.test.tsx, downloadFile.test.ts and MappingRepository.test.ts.
  useExportMapping: () => ({ mutateAsync: vi.fn(), isPending: false }),
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

const messageSuccess = vi.hoisted(() => vi.fn());
const messageError = vi.hoisted(() => vi.fn());

// Only `message` is replaced; every other antd export (Table, Modal, Select,
// Segmented, ...) stays real, exactly like OltTable.test.tsx's use of this
// pattern to assert on which toast a mutation's outcome actually produced.
vi.mock("antd", async () => {
  const antd = await vi.importActual<typeof import("antd")>("antd");
  return {
    ...antd,
    message: { success: messageSuccess, error: messageError },
  };
});

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

  // The mirror of the node-edit test above (and of NetworkMapPage.delete's
  // wiring guards): a copy-paste of useUpdateNode into the cable row's Ubah
  // button — the closest, most likely mistake, since editNode/saveNode
  // already exist as the template — would pass everything else and only
  // show up here.
  it("edits a cable through Ubah, saving via useUpdateEdge rather than useUpdateNode", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    const edgeRow = screen.getByText("ODC-01--ODP-01").closest("tr")!;
    await userEvent.click(
      within(edgeRow).getByRole("button", { name: "Ubah" }),
    );
    expect(screen.getByText("Ubah kabel")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(updateEdgeMutateAsync).toHaveBeenCalledWith(
      expect.objectContaining({ edgeId: "ODC-01--ODP-01" }),
    );
    expect(updateNodeMutateAsync).not.toHaveBeenCalled();
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

  // useCableDraw.points holds only the tapped corners, never the two nodes'
  // own positions — the ordinary drop cable (node straight to node, no
  // corners) traces zero of them. Measuring `points` alone therefore saves a
  // length of 0 for the single most common cable in the network.
  it("saves a nonzero length for a straight cable with no corners", async () => {
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    const [[saved]] = createEdgeMutateAsync.mock.calls;
    expect(saved.distance).toBeGreaterThan(0);
  });

  it("records a longer length when the cable detours through a corner than the straight line between its ends", async () => {
    render(<NetworkMapPage />);

    // Baseline: the same two nodes, traced with no corner at all.
    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    const straightDistance = createEdgeMutateAsync.mock.calls[0][0].distance;

    // Same pair, this time detouring through a corner well off the direct line.
    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onDrop({ lat: -6.15, lng: 106.9 }));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));
    const detourDistance = createEdgeMutateAsync.mock.calls[1][0].distance;

    expect(detourDistance).toBeGreaterThan(straightDistance);
  });

  it("tells the operator plainly that the cable already exists on a 409", async () => {
    createEdgeMutateAsync.mockRejectedValueOnce(
      new ApiError(409, "EDGE_EXISTS"),
    );
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith(
        "Sudah ada kabel antara kedua node ini",
      ),
    );
  });

  // The capacity rule requirement 1 exists to make reachable (odp_to_odp /
  // odc_to_odc) would be misreported as a duplicate cable if every 409 showed
  // the same "already exists" text. SLOTS_FULL — not a fabricated 422 code —
  // is what the backend actually sends for this (mapping_handler.go); a
  // fixture using any other status or code would pass without ever
  // exercising the branch it claims to guard.
  it("surfaces the backend's own message for a failure that is not a duplicate id", async () => {
    createEdgeMutateAsync.mockRejectedValueOnce(
      new ApiError(
        409,
        "SLOTS_FULL",
        undefined,
        'slot penuh: "ODC-01" sudah penuh (8/8)',
      ),
    );
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: /Tarik kabel/ }));
    act(() => canvasProps.onNodeClick("ODC-01"));
    act(() => canvasProps.onNodeClick("ODP-01"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith(
        'slot penuh: "ODC-01" sudah penuh (8/8)',
      ),
    );
    expect(messageError).not.toHaveBeenCalledWith(
      "Sudah ada kabel antara kedua node ini",
    );
  });

  // saveCable already tells a duplicate id from every other failure; saveNode
  // reported both alike as "Gagal menyimpan node", which sent an operator who
  // just needed a different code hunting for a network problem instead.
  it("tells the operator a node's code is already taken, not a generic save failure", async () => {
    createNodeMutateAsync.mockRejectedValueOnce(
      new ApiError(409, "NODE_EXISTS"),
    );
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByRole("button", { name: "+ ODP" }));
    act(() => canvasProps.onDrop({ lat: -6.001, lng: 106.001 }));
    await userEvent.type(screen.getByLabelText("Nama"), "ODP Baru");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith(
        "Kode node sudah dipakai, gunakan kode lain",
      ),
    );
  });

  // UpdateNode's coordinate check on a mirror node (mapping_nodes.go) answers
  // this code with an English message (validateCoordinates, shared with the
  // site/OLT forms) — it must not reach the operator verbatim.
  it("shows an Indonesian message when a node's coordinates are refused, not the backend's English text", async () => {
    updateNodeMutateAsync.mockRejectedValueOnce(
      new ApiError(
        400,
        "INVALID_COORDINATES",
        undefined,
        "latitude 200 is outside -90..90",
      ),
    );
    render(<NetworkMapPage />);

    await userEvent.click(screen.getByText("Daftar"));
    await userEvent.click(screen.getAllByRole("button", { name: "Ubah" })[0]);
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    await waitFor(() =>
      expect(messageError).toHaveBeenCalledWith("Koordinat tidak valid"),
    );
  });
});
