import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CableTypeModal } from "./CableTypeModal";
import { FIBER_LABELS } from "./mappingLabels";

// Trimming the options down to a couple of entries would still pass a test
// that only checked two of the seven; this checks all of them by name so a
// missing cascade (odp_to_odp, odc_to_odc, their _ratio siblings) cannot slip
// through silently.
const OTHER_LABELS = Object.values(FIBER_LABELS).filter(
  (label) => label !== "Distribusi",
);

describe("CableTypeModal", () => {
  it("defaults to distribution but offers every one of the seven fiber types", async () => {
    render(<CableTypeModal open onCancel={() => {}} onSubmit={() => {}} />);

    expect(screen.getByText("Distribusi")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("combobox"));

    for (const label of OTHER_LABELS) {
      expect(await screen.findByTitle(label)).toBeInTheDocument();
    }
  });

  // The regression this whole task exists to prevent: a cable saved with
  // whatever the operator picked, not the default the modal opened with.
  it("saves the type the operator actually picked, not the default", async () => {
    const onSubmit = vi.fn();
    render(<CableTypeModal open onCancel={() => {}} onSubmit={onSubmit} />);

    await userEvent.click(screen.getByRole("combobox"));
    await userEvent.click(await screen.findByTitle("ODP ke ODP"));
    await userEvent.click(screen.getByRole("button", { name: "Simpan" }));

    expect(onSubmit).toHaveBeenCalledWith("odp_to_odp");
    expect(onSubmit).not.toHaveBeenCalledWith("distribution");
  });
});
