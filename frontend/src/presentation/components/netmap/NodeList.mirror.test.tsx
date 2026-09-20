import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import type { MappingNode } from "@/domain/entities";
import { NodeList } from "./NodeList";

// A server node that mirrors an OLT, as it comes back from the API once the
// OLT has coordinates (see MappingNode.oltId). The Daftar table is the
// second surface (after NodePopup) that offers to delete a node, and the
// rule — an OLT-backed node cannot be deleted from anywhere on the map —
// has to hold here too, not just at the popup.
const mirrorNode: MappingNode = {
  nodeId: "SERVER-OLT-01",
  oltId: "11111111-1111-1111-1111-111111111111",
  type: "server",
  name: "OLT-01",
  latitude: -6.21,
  longitude: 106.81,
  capacity: 0,
  splitter: "",
  pppoe: "",
  serialNumber: "",
  notes: "",
};

const ordinaryNode: MappingNode = {
  nodeId: "ODP-01",
  type: "odp",
  name: "ODP Satu",
  latitude: -6.2,
  longitude: 106.8,
  capacity: 8,
  splitter: "",
  pppoe: "",
  serialNumber: "",
  notes: "",
};

function renderList(nodes: MappingNode[]) {
  render(
    <MemoryRouter>
      <NodeList nodes={nodes} onEdit={vi.fn()} onDelete={vi.fn()} />
    </MemoryRouter>,
  );
}

describe("NodeList row actions for a node that mirrors an OLT", () => {
  // Two-sided on purpose: a version that hides Hapus from every row would
  // pass a one-sided assertion just as easily as the correct fix.
  it("offers no Hapus for a mirror row, but still offers it for an ordinary row", () => {
    renderList([mirrorNode, ordinaryNode]);

    expect(screen.getAllByRole("button", { name: "Hapus" })).toHaveLength(1);
  });

  it("points a mirror row at the OLT menu instead", () => {
    renderList([mirrorNode]);

    const link = screen.getByRole("link", { name: /menu OLT/i });
    expect(link).toHaveAttribute("href", "/olts");
  });
});
