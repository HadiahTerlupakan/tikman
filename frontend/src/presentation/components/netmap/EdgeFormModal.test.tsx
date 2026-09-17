import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingEdge } from "@/domain/entities";
import { EdgeFormModal } from "./EdgeFormModal";

// An existing cable, as it would come back from the API — geometry already
// traced, unlike a fresh one still being drawn.
const existingEdge: MappingEdge = {
  id: "e1",
  edgeId: "ODC-01--ODP-01",
  source: "ODC-01",
  target: "ODP-01",
  fiberType: "distribution",
  distance: 120,
  waypoints: [{ lat: -6.2, lng: 106.8 }],
  notes: "Kabel lama",
};

describe("EdgeFormModal", () => {
  it("shows the cable's own fiber type and notes, not a blank form", () => {
    render(
      <EdgeFormModal
        open
        initial={existingEdge}
        onCancel={() => {}}
        onSubmit={() => {}}
      />,
    );

    expect(screen.getByText("Distribusi")).toBeInTheDocument();
    expect(screen.getByLabelText("Catatan")).toHaveValue("Kabel lama");
  });

  // The only two fields this modal may change. Asserting the full merged
  // object (not just the two fields) is what proves source, target,
  // distance and waypoints survive untouched — those are geometry and
  // belong to redrawing the cable on the map, not this table-row edit.
  it("submits the changed fiber type and notes, leaving the cable's geometry untouched", async () => {
    const onSubmit = vi.fn();
    render(
      <EdgeFormModal
        open
        initial={existingEdge}
        onCancel={() => {}}
        onSubmit={onSubmit}
      />,
    );

    await userEvent.click(screen.getByRole("combobox"));
    await userEvent.click(await screen.findByTitle("ODP ke ODP"));
    await userEvent.clear(screen.getByLabelText("Catatan"));
    await userEvent.type(screen.getByLabelText("Catatan"), "Sudah diperiksa");
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith({
      ...existingEdge,
      fiberType: "odp_to_odp",
      notes: "Sudah diperiksa",
    });
  });
});
