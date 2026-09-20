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

// Neither field makes sense for its own type, but the model carries all four
// on every node regardless — a value that snuck in some other way (a manual
// DB edit, a future feature) must survive an edit of a field this form never
// shows for that type.
const existingOntWithStrayFields: MappingNode = {
  nodeId: "ONT-CONTOH-01",
  type: "ont",
  name: "ONT Contoh",
  latitude: -6.21,
  longitude: 106.81,
  capacity: 4,
  splitter: "1:8",
  pppoe: "pelanggan@pppoe",
  serialNumber: "ZTEGC0000099",
  notes: "",
};

const existingOdpWithStrayFields: MappingNode = {
  nodeId: "ODP-CONTOH-02",
  type: "odp",
  name: "ODP Contoh Dua",
  latitude: -6.21,
  longitude: 106.81,
  capacity: 8,
  splitter: "1:8",
  pppoe: "nyasar@pppoe",
  serialNumber: "SERIAL-NYASAR",
  notes: "",
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

  // A regression that scrambled a field, or forgot the string-to-number
  // conversion on the coordinates, would still pass every other test here.
  it("submits a complete new node, with numeric coordinates", async () => {
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
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledTimes(1);
    const submitted = onSubmit.mock.calls[0][0];
    expect(submitted).toMatchObject({
      type: "odp",
      name: "ODP Baru",
      latitude: -6.21,
      longitude: 106.81,
      capacity: 0,
      splitter: "",
      pppoe: "",
      serialNumber: "",
      notes: "",
    });
    // The form holds these as text so the read-only inputs can display them;
    // "-6.21" == -6.21 loosely, which would hide a lost Number() conversion.
    expect(typeof submitted.latitude).toBe("number");
    expect(typeof submitted.longitude).toBe("number");
    expect(typeof submitted.nodeId).toBe("string");
    expect(submitted.nodeId.length).toBeGreaterThan(0);
  });

  it("asks an ONT for what only an ONT has, and nothing else", () => {
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
    expect(screen.queryByLabelText("Jumlah slot")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Rasio splitter")).not.toBeInTheDocument();
  });

  it("offers every splitter ratio this network stocks, and nothing to type", async () => {
    const user = userEvent.setup();
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    await user.click(screen.getByLabelText("Rasio splitter"));

    for (const ratio of ["1:2", "1:4", "1:8", "1:16"]) {
      expect(await screen.findByTitle(ratio)).toBeInTheDocument();
    }
  });

  it("asks an OLT for nothing a splitter would answer, but still asks an ODC", () => {
    const { unmount } = render(
      <NodeFormModal
        open
        type="server"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.queryByLabelText("Jumlah slot")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Rasio splitter")).not.toBeInTheDocument();
    unmount();

    render(
      <NodeFormModal
        open
        type="odc"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByLabelText("Jumlah slot")).toBeInTheDocument();
    expect(screen.getByLabelText("Rasio splitter")).toBeInTheDocument();
  });

  it("keeps ONT-only fields off a non-ONT form", () => {
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: -6.21, lng: 106.81 }}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.queryByLabelText("PPPoE")).not.toBeInTheDocument();
    expect(screen.queryByLabelText("Serial")).not.toBeInTheDocument();
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

  it("submits the complete edited node unchanged, when nothing was retyped", async () => {
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

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith({
      id: undefined,
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
    });
  });

  // The toolbar's `type` is only ever a guess at what is being placed next;
  // for a node that already exists, that node's own type is the fact of the
  // matter. Losing it would also silently gate away — and zero out — the
  // ODP-only fields this test pins.
  it("keeps the node's own type, capacity and splitter when they disagree with the placement type", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="ont"
        position={{ lat: 0, lng: 0 }}
        initial={existingOdp}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        type: "odp",
        capacity: 8,
        splitter: "1:8",
      }),
    );
  });

  // effectiveType gates capacity/splitter off an ONT's form and pppoe/serial
  // off every other type's — values that field never rendered still went
  // through the submit ?? default and were zeroed, even when nothing was
  // retyped.
  it("keeps an ONT's own capacity and splitter when editing, though the form never shows them", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="ont"
        position={{ lat: 0, lng: 0 }}
        initial={existingOntWithStrayFields}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ capacity: 4, splitter: "1:8" }),
    );
  });

  it("keeps a non-ONT node's own PPPoE and serial when editing, though the form never shows them", async () => {
    const onSubmit = vi.fn();
    render(
      <NodeFormModal
        open
        type="odp"
        position={{ lat: 0, lng: 0 }}
        initial={existingOdpWithStrayFields}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        pppoe: "nyasar@pppoe",
        serialNumber: "SERIAL-NYASAR",
      }),
    );
  });
});
