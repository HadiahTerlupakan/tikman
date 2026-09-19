import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { EdgePopup } from "./EdgePopup";

function node(
  overrides: Partial<MappingNode> & { nodeId: string },
): MappingNode {
  return {
    type: "odp",
    name: overrides.nodeId,
    latitude: -6.2,
    longitude: 106.8,
    capacity: 0,
    splitter: "",
    pppoe: "",
    serialNumber: "",
    notes: "",
    ...overrides,
  };
}

function edge(overrides: Partial<MappingEdge> = {}): MappingEdge {
  return {
    edgeId: "ODC-01--ODP-01",
    source: "ODC-01",
    target: "ODP-01",
    fiberType: "distribution",
    distance: 120,
    waypoints: [],
    notes: "",
    ...overrides,
  };
}

const sourceNode = node({ nodeId: "ODC-01", name: "ODP Masjid" });
const targetNode = node({ nodeId: "ODP-01", name: "Rumah Pak Budi" });

describe("EdgePopup", () => {
  it("reads both ends by their node name as its heading, not their id", () => {
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "ODP Masjid → Rumah Pak Budi" }),
    ).toBeInTheDocument();
  });

  it("shows the ids secondary to the names", () => {
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("ODC-01 → ODP-01")).toBeInTheDocument();
  });

  it("shows the fiber type and the drawn length", () => {
    render(
      <EdgePopup
        edge={edge({ fiberType: "odp_to_odp", distance: 1500 })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("ODP ke ODP")).toBeInTheDocument();
    // formatMeters switches to km above 1000m — proves the popup calls the
    // shared formatter rather than printing the raw metre count itself.
    expect(screen.getByText("1,50 km")).toBeInTheDocument();
  });

  it("shows notes only when there are any", () => {
    const { rerender } = render(
      <EdgePopup
        edge={edge({ notes: "" })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.queryByText("Catatan")).not.toBeInTheDocument();

    rerender(
      <EdgePopup
        edge={edge({ notes: "Kabel lama" })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("Catatan")).toBeInTheDocument();
    expect(screen.getByText("Kabel lama")).toBeInTheDocument();
  });

  // Node deletion never cascades to edges (migration 53's own design), so a
  // cable can point at a node that is gone. This must read as honest, not
  // blank, not a raw id standing in for a name, and never a crash.
  it("names a deleted source node honestly instead of leaving it blank or crashing", () => {
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={undefined}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Node sudah dihapus → Rumah Pak Budi"),
    ).toBeInTheDocument();
    expect(screen.getByText("ODC-01 → ODP-01")).toBeInTheDocument();
  });

  it("names a deleted target node honestly instead of leaving it blank or crashing", () => {
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={sourceNode}
        targetNode={undefined}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText("ODP Masjid → Node sudah dihapus"),
    ).toBeInTheDocument();
  });

  it("asks to edit this exact cable, then closes the popup", async () => {
    const onEdit = vi.fn();
    const onClose = vi.fn();
    const target = edge();
    render(
      <EdgePopup
        edge={target}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={onEdit}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(onEdit).toHaveBeenCalledWith(target);
    expect(onClose).toHaveBeenCalled();
  });

  it("starts a redraw of this exact cable, then closes the popup", async () => {
    const onRedraw = vi.fn();
    const onClose = vi.fn();
    const target = edge();
    render(
      <EdgePopup
        edge={target}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={onRedraw}
        onDelete={vi.fn()}
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Gambar ulang" }));

    expect(onRedraw).toHaveBeenCalledWith(target);
    expect(onClose).toHaveBeenCalled();
  });

  it("requires confirming Hapus before deleting", async () => {
    const onDelete = vi.fn();
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={onDelete}
        onClose={vi.fn()}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    expect(onDelete).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));
    expect(onDelete).toHaveBeenCalledWith("ODC-01--ODP-01");
  });

  it("closes the popup once the delete is confirmed", async () => {
    const onClose = vi.fn();
    render(
      <EdgePopup
        edge={edge()}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(onClose).toHaveBeenCalled();
  });
});
