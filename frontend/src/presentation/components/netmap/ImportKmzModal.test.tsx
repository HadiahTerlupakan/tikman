import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type {
  ImportedEdge,
  ImportedNode,
  ImportPreview,
} from "@/domain/entities";
import { ImportKmzModal } from "./ImportKmzModal";

const previewMutateAsync = vi.fn();
const commitMutateAsync = vi.fn();
vi.mock("@/application/hooks", () => ({
  usePreviewImport: () => ({
    mutateAsync: previewMutateAsync,
    isPending: false,
  }),
  useCommitImport: () => ({ mutateAsync: commitMutateAsync, isPending: false }),
}));
vi.mock("antd", async (importOriginal) => {
  const actual = await importOriginal<typeof import("antd")>();
  return {
    ...actual,
    message: { ...actual.message, success: vi.fn(), error: vi.fn() },
  };
});

function baseNode(overrides: Partial<ImportedNode> = {}): ImportedNode {
  return {
    row: 1,
    nodeId: "ODP-01",
    type: "odp",
    name: "ODP Satu",
    latitude: -6.2,
    longitude: 106.8,
    capacity: 0,
    splitter: "",
    pppoe: "",
    serialNumber: "",
    notes: "",
    reason: "Dari data ekspor (ExtendedData)",
    conflict: false,
    conflictReason: "",
    blocked: false,
    blockedReason: "",
    include: true,
    ...overrides,
  };
}

function previewWith(nodes: ImportedNode[]): ImportPreview {
  return { nodes, edges: [], issues: [], totalPlacemarks: nodes.length };
}

function baseEdge(overrides: Partial<ImportedEdge> = {}): ImportedEdge {
  return {
    row: 1,
    edgeId: "E-1",
    source: "",
    target: "",
    fiberType: "",
    distance: 0,
    waypoints: null,
    notes: "",
    reason: "Ujung kabel tidak dekat node manapun pada berkas ini; isi manual.",
    conflict: false,
    conflictReason: "",
    unresolved: true,
    include: false,
    ...overrides,
  };
}

// Chooses a file through the real <input type="file"> antd's Upload
// renders, then presses the explicit "Pratinjau" button - mirrors
// BroadcastModal.test.tsx's own upload idiom (fireEvent, not userEvent.upload,
// since the assertions after it are synchronous).
async function uploadAndPreview(file = new File(["isi"], "peta.kmz")) {
  const input = document.body.querySelector(
    'input[type="file"]',
  ) as HTMLInputElement;
  fireEvent.change(input, { target: { files: [file] } });
  await userEvent.click(screen.getByRole("button", { name: "Pratinjau" }));
}

describe("ImportKmzModal", () => {
  beforeEach(() => vi.clearAllMocks());

  it("keeps the preview button dead until a file is chosen, and writes nothing on its own", () => {
    render(<ImportKmzModal open onClose={vi.fn()} />);

    expect(screen.getByRole("button", { name: "Pratinjau" })).toBeDisabled();
    expect(previewMutateAsync).not.toHaveBeenCalled();
  });

  it("asks for a preview with the chosen file and renders what came back", async () => {
    const file = new File(["isi"], "peta.kmz");
    previewMutateAsync.mockResolvedValue(previewWith([baseNode()]));
    render(<ImportKmzModal open onClose={vi.fn()} />);

    await uploadAndPreview(file);

    expect(previewMutateAsync).toHaveBeenCalledWith(file);
    expect(await screen.findByDisplayValue("ODP-01")).toBeInTheDocument();
    expect(screen.getByText(/1 placemark ditemukan/)).toBeInTheDocument();
  });

  // A server node mirrors a real OLT and can never be created by import - the
  // preview must say so instead of offering a checkbox nobody may check.
  it("shows a blocked node as unselectable rather than as an editable row", async () => {
    previewMutateAsync.mockResolvedValue(
      previewWith([
        baseNode({
          nodeId: "SERVER-01",
          type: "server",
          blocked: true,
          blockedReason:
            "Node server mengikuti data OLT dan dibuat lewat menu OLT, bukan lewat impor",
          include: false,
        }),
      ]),
    );
    render(<ImportKmzModal open onClose={vi.fn()} />);

    await uploadAndPreview();

    expect(await screen.findByText("Tidak bisa diimpor")).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });

  // The button must reflect what will really be committed, not what the
  // preview first suggested - unchecking the only included row must disable it.
  it("disables committing once every row has been unchecked", async () => {
    previewMutateAsync.mockResolvedValue(previewWith([baseNode()]));
    render(<ImportKmzModal open onClose={vi.fn()} />);
    await uploadAndPreview();
    const commitButton = await screen.findByRole("button", {
      name: "Simpan ke Peta",
    });
    expect(commitButton).toBeEnabled();

    await userEvent.click(screen.getByRole("checkbox"));

    expect(commitButton).toBeDisabled();
  });

  // The whole point of an editable preview: a correction made on screen must
  // be exactly what reaches the backend, not the file's original guess.
  it("commits the edited node id, not the one the file originally carried", async () => {
    previewMutateAsync.mockResolvedValue(
      previewWith([baseNode({ nodeId: "ODP-SALAH" })]),
    );
    commitMutateAsync.mockResolvedValue({ nodesCreated: 1, edgesCreated: 0 });
    const onClose = vi.fn();
    render(<ImportKmzModal open onClose={onClose} />);
    await uploadAndPreview();

    const codeInput = await screen.findByDisplayValue("ODP-SALAH");
    await userEvent.clear(codeInput);
    await userEvent.type(codeInput, "ODP-01");
    await userEvent.click(
      screen.getByRole("button", { name: "Simpan ke Peta" }),
    );

    expect(commitMutateAsync).toHaveBeenCalledWith({
      nodes: [expect.objectContaining({ nodeId: "ODP-01" })],
      edges: [],
    });
    expect(onClose).toHaveBeenCalled();
  });

  // The commonest defect in a hand-made or third-party file is lat/lng
  // swapped - and the person confirming the preview cannot correct, or even
  // notice, a coordinate they are never shown.
  it("shows the node's coordinates and commits an edit made to them", async () => {
    previewMutateAsync.mockResolvedValue(previewWith([baseNode()]));
    commitMutateAsync.mockResolvedValue({ nodesCreated: 1, edgesCreated: 0 });
    render(<ImportKmzModal open onClose={vi.fn()} />);
    await uploadAndPreview();

    expect(await screen.findByDisplayValue("-6.2")).toBeInTheDocument();
    expect(screen.getByDisplayValue("106.8")).toBeInTheDocument();

    // fireEvent, not userEvent.type: antd's InputNumber treats each
    // intermediate keystroke of a negative decimal ("-", "-6", "-6.")  as an
    // invalid number and resets, the same reason BroadcastModal.test.tsx's
    // own file input uses fireEvent rather than simulating keystrokes.
    const latitudeInput = screen.getByDisplayValue("-6.2");
    fireEvent.change(latitudeInput, { target: { value: "-6.25" } });
    await userEvent.click(
      screen.getByRole("button", { name: "Simpan ke Peta" }),
    );

    expect(commitMutateAsync).toHaveBeenCalledWith({
      nodes: [expect.objectContaining({ latitude: -6.25 })],
      edges: [],
    });
  });

  // Kapasitas already has min={0}; the coordinate columns did not carry
  // any bound at all, so a value like 999 or -5000 could be typed and
  // would only be refused server-side, after submission rather than before.
  it("bounds the coordinate inputs to real latitude/longitude ranges", async () => {
    previewMutateAsync.mockResolvedValue(previewWith([baseNode()]));
    render(<ImportKmzModal open onClose={vi.fn()} />);
    await uploadAndPreview();

    const latitudeInput = screen.getByDisplayValue("-6.2");
    const longitudeInput = screen.getByDisplayValue("106.8");

    expect(latitudeInput).toHaveAttribute("aria-valuemin", "-90");
    expect(latitudeInput).toHaveAttribute("aria-valuemax", "90");
    expect(longitudeInput).toHaveAttribute("aria-valuemin", "-180");
    expect(longitudeInput).toHaveAttribute("aria-valuemax", "180");
  });

  // Every imported node lands at capacity 0, which checkSlots reads as
  // "nobody has counted the ports yet" and never enforces - a capacity
  // nobody can see or set in the preview is a capacity rule that never
  // applies to anything imported.
  it("shows the node's capacity and commits a value entered for it", async () => {
    previewMutateAsync.mockResolvedValue(
      previewWith([baseNode({ capacity: 0 })]),
    );
    commitMutateAsync.mockResolvedValue({ nodesCreated: 1, edgesCreated: 0 });
    render(<ImportKmzModal open onClose={vi.fn()} />);
    await uploadAndPreview();

    const capacityInput = screen.getByDisplayValue("0");
    fireEvent.change(capacityInput, { target: { value: "8" } });
    await userEvent.click(
      screen.getByRole("button", { name: "Simpan ke Peta" }),
    );

    expect(commitMutateAsync).toHaveBeenCalledWith({
      nodes: [expect.objectContaining({ capacity: 8 })],
      edges: [],
    });
  });

  it("surfaces unsupported placemarks as issues rather than dropping them silently", async () => {
    previewMutateAsync.mockResolvedValue({
      nodes: [],
      edges: [],
      issues: [
        {
          row: 1,
          name: "Area Layanan",
          folder: "Lain",
          reason: "Bentuk tidak didukung (bukan titik atau garis)",
        },
      ],
      totalPlacemarks: 1,
    });
    render(<ImportKmzModal open onClose={vi.fn()} />);

    await uploadAndPreview();

    const alert = await screen.findByText(/1 placemark tidak dikenali/);
    expect(
      within(alert.closest(".ant-alert") as HTMLElement).getByText(
        /Area Layanan/,
      ),
    ).toBeInTheDocument();
  });

  // migrations/53_network_mapping.sql: a cable is legal without its ends
  // resolved yet - the row must say so and stay unchecked, not be silently
  // dropped or silently included with a blank endpoint.
  it("shows an edge with no resolvable endpoint as unchecked and unresolved", async () => {
    previewMutateAsync.mockResolvedValue({
      nodes: [],
      edges: [baseEdge()],
      issues: [],
      totalPlacemarks: 1,
    });
    render(<ImportKmzModal open onClose={vi.fn()} />);

    await uploadAndPreview();

    expect(await screen.findByText(/Belum lengkap/)).toBeInTheDocument();
    expect(screen.getByRole("checkbox")).not.toBeChecked();
  });

  // Filling in the two ends by hand and checking the box is exactly how a
  // field survey's bare lines (no ExtendedData, nothing to guess from) get
  // imported at all.
  it("commits a manually completed edge once its endpoints are filled in and it is checked", async () => {
    previewMutateAsync.mockResolvedValue({
      nodes: [],
      edges: [baseEdge()],
      issues: [],
      totalPlacemarks: 1,
    });
    commitMutateAsync.mockResolvedValue({ nodesCreated: 0, edgesCreated: 1 });
    render(<ImportKmzModal open onClose={vi.fn()} />);
    await uploadAndPreview();

    await screen.findByText(/Belum lengkap/);
    const inputs = screen.getAllByRole("textbox"); // Kode, Sumber, Tujuan, in column order
    await userEvent.type(inputs[1], "ODC-01");
    await userEvent.type(inputs[2], "ODP-01");
    await userEvent.click(screen.getByRole("checkbox"));
    await userEvent.click(
      screen.getByRole("button", { name: "Simpan ke Peta" }),
    );

    expect(commitMutateAsync).toHaveBeenCalledWith({
      nodes: [],
      edges: [
        expect.objectContaining({
          source: "ODC-01",
          target: "ODP-01",
          include: true,
        }),
      ],
    });
  });
});
