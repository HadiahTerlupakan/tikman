import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingEdge } from "@/domain/entities";
import { EdgeList } from "./EdgeList";

const edge: MappingEdge = {
  edgeId: "ODC-01--ODP-01",
  source: "ODC-01",
  target: "ODP-01",
  fiberType: "odp_to_odp",
  distance: 1500,
  waypoints: [],
  notes: "",
};

describe("EdgeList", () => {
  // A mutant that renders the raw fiberType or the raw metre count would still
  // pass a test that only checked the row existed; pinning the Indonesian
  // label and the formatted length catches exactly that.
  it("shows the cable in Indonesian, not as raw stored values", () => {
    render(<EdgeList edges={[edge]} onDelete={vi.fn()} onEdit={vi.fn()} />);

    expect(screen.getByText("ODC-01--ODP-01")).toBeInTheDocument();
    expect(screen.getByText("ODC-01")).toBeInTheDocument();
    expect(screen.getByText("ODP-01")).toBeInTheDocument();
    expect(screen.getByText("ODP ke ODP")).toBeInTheDocument();
    expect(screen.queryByText("odp_to_odp")).not.toBeInTheDocument();
    expect(screen.getByText("1,50 km")).toBeInTheDocument();
    expect(screen.queryByText("1500")).not.toBeInTheDocument();
  });

  it("asks the caller to delete the cable that was clicked, by its edge id", async () => {
    const onDelete = vi.fn();
    render(<EdgeList edges={[edge]} onDelete={onDelete} onEdit={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));

    expect(onDelete).toHaveBeenCalledWith("ODC-01--ODP-01");
  });

  it("asks the caller to edit the cable that was clicked, with its full data", async () => {
    const onEdit = vi.fn();
    render(<EdgeList edges={[edge]} onDelete={vi.fn()} onEdit={onEdit} />);

    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(onEdit).toHaveBeenCalledWith(edge);
  });
});
