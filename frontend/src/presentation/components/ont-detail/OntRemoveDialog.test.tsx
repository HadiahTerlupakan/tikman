import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { Ont } from "@/domain/entities";
import { OntRemoveDialog } from "./OntRemoveDialog";

vi.mock("@/application/hooks/useOnts", () => ({
  useOntRemovalPreview: () => ({
    data: ["interface gpon-olt_1/3/1", "no onu 15", "exit"],
    isLoading: false,
    error: null,
  }),
}));

const ont = {
  id: "ont-1",
  serialNumber: "HWTCB403E8A0",
} as Ont;

function renderDialog(onConfirm = vi.fn()) {
  render(
    <OntRemoveDialog ont={ont} open onCancel={vi.fn()} onConfirm={onConfirm} />,
  );
  return onConfirm;
}

describe("OntRemoveDialog", () => {
  // Removing from the OLT writes to a live device, so what will be sent is
  // shown before the operator agrees to it.
  it("shows the commands it would send to the OLT", () => {
    renderDialog();

    expect(screen.getByTestId("removal-commands")).toHaveTextContent(
      "no onu 15",
    );
  });

  it("removes from the OLT by default", async () => {
    const onConfirm = renderDialog();

    await userEvent.click(screen.getByRole("button", { name: "Remove" }));

    expect(onConfirm).toHaveBeenCalledWith(true);
  });

  // Clearing the box keeps the OLT untouched, and the dialog has to say what
  // that leaves behind rather than let the operator assume the ONU is gone.
  it("warns that the ONU stays on the OLT when unchecked", async () => {
    const onConfirm = renderDialog();

    await userEvent.click(screen.getByRole("checkbox"));

    expect(screen.getByText(/stays configured on the OLT/)).toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: "Remove" }));
    expect(onConfirm).toHaveBeenCalledWith(false);
  });
});
