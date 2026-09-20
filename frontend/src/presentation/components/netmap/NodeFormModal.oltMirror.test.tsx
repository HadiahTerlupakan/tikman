import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingNode } from "@/domain/entities";
import { NodeFormModal } from "./NodeFormModal";

// A server node that mirrors an OLT, as it comes back from the API once the
// OLT has coordinates (see MappingNode.oltId and olt_map_node.go).
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

// UpdateNode silently pins name (and type) back to whatever the OLT record
// already says for any node carrying oltId (mapping_nodes.go) — offering an
// editable Nama field here would let an operator retype it, click Simpan,
// and watch the change vanish with no error explaining why.
describe("NodeFormModal on a node that mirrors an OLT", () => {
  it("will not let the OLT's name be retyped", () => {
    render(
      <NodeFormModal
        open
        type="server"
        position={{ lat: 0, lng: 0 }}
        initial={mirrorNode}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    const nameField = screen.getByLabelText("Nama") as HTMLInputElement;
    expect(nameField.readOnly).toBe(true);
  });

  it("keeps a plain node's Nama field editable, unaffected by the mirror guard", () => {
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: 0, lng: 0 }}
        initial={{ ...mirrorNode, oltId: undefined, type: "odp" }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    const nameField = screen.getByLabelText("Nama") as HTMLInputElement;
    expect(nameField.readOnly).toBe(false);
  });

  // The one thing that must still work: correcting where the device sits.
  it("still lets a mirror node's stored position be corrected", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="server"
        position={{ lat: 0, lng: 0 }}
        initial={mirrorNode}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );
    const latitudeField = screen.getByLabelText("Latitude");
    await userEvent.clear(latitudeField);
    await userEvent.type(latitudeField, "-6.99");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ latitude: -6.99, name: "OLT-01" }),
    );
  });
});
