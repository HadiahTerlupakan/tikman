import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { MappingNode } from "@/domain/entities";
import { NodeList } from "./NodeList";

const node: MappingNode = {
  nodeId: "ODP-01",
  type: "odp",
  name: "ODP Satu",
  latitude: -6.21,
  longitude: 106.81,
  capacity: 8,
  splitter: "1:8",
  pppoe: "",
  serialNumber: "",
  notes: "",
};

describe("NodeList", () => {
  // Mirrors EdgeList's own delete-confirmation test: one misclick in a
  // 20-row table used to remove a node outright, orphaning every cable drawn
  // to it with no undo.
  it("asks the caller to delete the node that was clicked, only after confirming", async () => {
    const onDelete = vi.fn();
    render(<NodeList nodes={[node]} onDelete={onDelete} onEdit={vi.fn()} />);

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    expect(onDelete).not.toHaveBeenCalled();

    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(onDelete).toHaveBeenCalledWith("ODP-01");
  });

  it("asks the caller to edit the node that was clicked, with its full data", async () => {
    const onEdit = vi.fn();
    render(<NodeList nodes={[node]} onDelete={vi.fn()} onEdit={onEdit} />);

    await userEvent.click(screen.getByRole("button", { name: "Ubah" }));

    expect(onEdit).toHaveBeenCalledWith(node);
  });
});
