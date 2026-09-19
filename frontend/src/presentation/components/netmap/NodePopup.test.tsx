import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingNode } from "@/domain/entities";
import { NodePopup } from "./NodePopup";

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

describe("NodePopup", () => {
  it("reads the node's name as its heading", () => {
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01", name: "ODP Depan Masjid" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("heading", { name: "ODP Depan Masjid" }),
    ).toBeInTheDocument();
  });

  it("shows the id secondary to the name, not in place of it", () => {
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01", name: "ODP Depan Masjid" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("ODP-01")).toBeInTheDocument();
  });

  // Each type must carry its OWN colour, not just "a" colour that differs
  // from some other type — a mapping that swapped two types' colours would
  // still pass a test that only checked one type, or only checked inequality.
  it("badges each node type in that type's own map colour, not another's", () => {
    const { rerender } = render(
      <NodePopup
        node={node({ nodeId: "SRV-01", type: "server" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("Server / OLT")).toHaveStyle({
      backgroundColor: "#8b5cf6",
    });

    rerender(
      <NodePopup
        node={node({ nodeId: "ODP-01", type: "odp" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );
    expect(screen.getByText("ODP")).toHaveStyle({ backgroundColor: "#06b6d4" });
  });

  it("formats coordinates readably rather than printing raw floats", () => {
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01", latitude: -6.2, longitude: 106.8 })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("-6.200000, 106.800000")).toBeInTheDocument();
  });

  // The core of this design: a node exists to be placed first and described
  // later, so most nodes will have most optional fields empty. A popup that
  // prints every field regardless — the exact mutant this feature's own
  // review history caught once already — would fail this.
  it("renders no optional-field row when none of them are filled", () => {
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByText("Kapasitas")).not.toBeInTheDocument();
    expect(screen.queryByText("Splitter")).not.toBeInTheDocument();
    expect(screen.queryByText("PPPoE")).not.toBeInTheDocument();
    expect(screen.queryByText("Serial")).not.toBeInTheDocument();
    expect(screen.queryByText("Catatan")).not.toBeInTheDocument();
  });

  it("renders only the optional fields that are actually filled", () => {
    render(
      <NodePopup
        node={node({
          nodeId: "ODP-01",
          capacity: 8,
          splitter: "1:8",
          pppoe: "",
          serialNumber: "",
          notes: "",
        })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Kapasitas")).toBeInTheDocument();
    expect(screen.getByText("8")).toBeInTheDocument();
    expect(screen.getByText("Splitter")).toBeInTheDocument();
    expect(screen.getByText("1:8")).toBeInTheDocument();
    expect(screen.queryByText("PPPoE")).not.toBeInTheDocument();
    expect(screen.queryByText("Serial")).not.toBeInTheDocument();
    expect(screen.queryByText("Catatan")).not.toBeInTheDocument();
  });

  it("asks to edit this exact node, then closes the popup", async () => {
    const onEdit = vi.fn();
    const onClose = vi.fn();
    const target = node({ nodeId: "ODP-01" });
    render(
      <NodePopup
        node={target}
        onEdit={onEdit}
        onDelete={vi.fn()}
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(onEdit).toHaveBeenCalledWith(target);
    expect(onClose).toHaveBeenCalled();
  });

  // Deleting a node from the map orphans every cable drawn to it, so a bare
  // click must not be enough — the same Popconfirm gate OltTable and
  // NodeList already use for every destructive control in this codebase.
  it("requires confirming Hapus before deleting", async () => {
    const onDelete = vi.fn();
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01" })}
        onEdit={vi.fn()}
        onDelete={onDelete}
        onClose={vi.fn()}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    expect(onDelete).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));
    expect(onDelete).toHaveBeenCalledWith("ODP-01");
  });

  it("closes the popup once the delete is confirmed", async () => {
    const onClose = vi.fn();
    render(
      <NodePopup
        node={node({ nodeId: "ODP-01" })}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={onClose}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(onClose).toHaveBeenCalled();
  });
});
