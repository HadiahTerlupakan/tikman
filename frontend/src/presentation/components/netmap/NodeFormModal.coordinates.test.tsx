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

// The spec asks for a checkbox to enter coordinates by hand; without it, a
// box tapped onto the wrong roof stays there forever, since delete-and-
// replace is the only other way to move it and that orphans every cable
// already drawn to it.
describe("NodeFormModal koordinat manual", () => {
  it("keeps the position read-only until the checkbox is ticked", async () => {
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

    const latitudeField = screen.getByLabelText("Latitude") as HTMLInputElement;
    expect(latitudeField.readOnly).toBe(true);

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );

    expect(latitudeField.readOnly).toBe(false);
  });

  // The case that matters most: correcting a box already on the map, not
  // just placing a new one.
  it("lets a node's stored position be corrected through the edit form", async () => {
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

    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );
    const latitudeField = screen.getByLabelText("Latitude");
    const longitudeField = screen.getByLabelText("Longitude");
    await userEvent.clear(latitudeField);
    await userEvent.type(latitudeField, "-6.9");
    await userEvent.clear(longitudeField);
    await userEvent.type(longitudeField, "107.5");

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ latitude: -6.9, longitude: 107.5 }),
    );
  });

  it("also lets a freshly placed node's coordinates be typed by hand, not just tapped", async () => {
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

    await userEvent.type(screen.getByLabelText("Nama"), "ODP Baru");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );
    const latitudeField = screen.getByLabelText("Latitude");
    await userEvent.clear(latitudeField);
    await userEvent.type(latitudeField, "-6.5");

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ latitude: -6.5, longitude: 106.81 }),
    );
  });

  it("refuses a latitude outside -90 to 90, in Indonesian", async () => {
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

    await userEvent.type(screen.getByLabelText("Nama"), "ODP Baru");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );
    const latitudeField = screen.getByLabelText("Latitude");
    await userEvent.clear(latitudeField);
    await userEvent.type(latitudeField, "120");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(
      await screen.findByText("Latitude harus di antara -90 dan 90"),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("refuses a longitude outside -180 to 180, in Indonesian", async () => {
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

    await userEvent.type(screen.getByLabelText("Nama"), "ODP Baru");
    await userEvent.click(
      screen.getByRole("checkbox", { name: "Isi koordinat manual" }),
    );
    const longitudeField = screen.getByLabelText("Longitude");
    await userEvent.clear(longitudeField);
    await userEvent.type(longitudeField, "200");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(
      await screen.findByText("Longitude harus di antara -180 dan 180"),
    ).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });
});
