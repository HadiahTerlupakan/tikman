import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MemoryRouter } from "react-router-dom";
import type { MappingNode } from "@/domain/entities";
import { NodePopup } from "./NodePopup";

// NodePopup fetches an ODP's subscriber count for real; irrelevant to a
// server node's popup and mocked to nothing here, same as NodePopup.test.tsx.
vi.mock("@/application/hooks", () => ({
  useOdpSubscribers: () => ({ data: undefined }),
}));

// A server node that mirrors an OLT, as it comes back from the API once the
// OLT has coordinates (see MappingNode.oltId).
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

function renderPopup(node: MappingNode, onDelete = vi.fn(), onEdit = vi.fn()) {
  const onClose = vi.fn();
  render(
    <MemoryRouter>
      <NodePopup
        node={node}
        edges={[]}
        nodesById={new Map()}
        onEdit={onEdit}
        onDelete={onDelete}
        onClose={onClose}
      />
    </MemoryRouter>,
  );
  return { onDelete, onEdit, onClose };
}

describe("NodePopup on a node that mirrors an OLT", () => {
  // Deleting here would read as removing a pin but would actually destroy
  // the OLT record and every ONT under it — DeleteNode already refuses this
  // server-side with NODE_MIRRORS_OLT (mapping_nodes.go).
  it("does not offer Hapus", () => {
    renderPopup(mirrorNode);

    expect(
      screen.queryByRole("button", { name: "Hapus" }),
    ).not.toBeInTheDocument();
  });

  // The backend's own refusal message points here too — the map should not
  // wait for that 409 round trip to say the same thing.
  it("points to the OLT menu instead", () => {
    renderPopup(mirrorNode);

    const link = screen.getByRole("link", { name: /menu OLT/i });
    expect(link).toHaveAttribute("href", "/olts");
  });

  // Moving stays available: correcting where a device sits is exactly what a
  // map is for, and UpdateNode already writes it back to the OLT record.
  it("still offers Ubah, to correct where the OLT sits", async () => {
    const { onEdit, onClose } = renderPopup(mirrorNode);

    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(onEdit).toHaveBeenCalledWith(mirrorNode);
    expect(onClose).toHaveBeenCalled();
  });

  // Regression guard: a plain node (no oltId) must keep its ordinary Hapus,
  // not lose it to a mutant that disables deletion unconditionally.
  it("still offers Hapus for an ordinary node that mirrors nothing", () => {
    renderPopup({ ...mirrorNode, oltId: undefined, type: "odp" });

    expect(screen.getByRole("button", { name: "Hapus" })).toBeInTheDocument();
    expect(
      screen.queryByRole("link", { name: /menu OLT/i }),
    ).not.toBeInTheDocument();
  });
});
