import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingNode } from "@/domain/entities";
import { NodeFormModal } from "./NodeFormModal";

// An existing ODP, as it would come back from the API — every field filled,
// unlike a freshly placed node.
const existingOdp: MappingNode = {
  nodeId: "ODP-CONTOH-01",
  type: "odp",
  name: "ODP Contoh Lama",
  latitude: -6.21,
  longitude: 106.81,
  capacity: 8,
  splitter: "1:8",
  pppoe: "",
  serialNumber: "",
  notes: "Dekat gapura",
};

describe("NodeFormModal", () => {
  // The position comes from the tap on the map, so the technician never types
  // a coordinate.
  it("fills the position from where the map was tapped", () => {
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByLabelText("Latitude")).toHaveValue("-6.21");
    expect(screen.getByLabelText("Longitude")).toHaveValue("106.81");
  });

  it("will not save a box with no name", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("asks an ONT for what only an ONT has", () => {
    render(
      <NodeFormModal
        open
        type="ont"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByLabelText("PPPoE")).toBeInTheDocument();
    expect(screen.getByLabelText("Serial")).toBeInTheDocument();
  });

  // Mode ubah: every field starts from the node being edited, not from the
  // map tap, and the title says so.
  it("shows the existing node's values when editing, instead of a blank form", () => {
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: 0, lng: 0 }}
        initial={existingOdp}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByText("Ubah node")).toBeInTheDocument();
    expect(screen.getByLabelText("Nama")).toHaveValue("ODP Contoh Lama");
    expect(screen.getByLabelText("Latitude")).toHaveValue("-6.21");
  });

  // node_id is what every cable's source/target points at; letting it be
  // retyped would silently orphan those cables.
  it("will not let node_id be retyped when editing", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: 0, lng: 0 }}
        initial={existingOdp}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    const nodeIdField = screen.getByLabelText("Kode") as HTMLInputElement;
    expect(nodeIdField.value).toBe("ODP-CONTOH-01");
    expect(nodeIdField.readOnly).toBe(true);

    await userEvent.type(nodeIdField, "GANTI-ID");
    expect(nodeIdField.value).toBe("ODP-CONTOH-01");

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ nodeId: "ODP-CONTOH-01" }),
    );
  });
});
