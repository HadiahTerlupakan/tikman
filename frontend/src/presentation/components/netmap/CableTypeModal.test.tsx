import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CableTypeModal } from "./CableTypeModal";

describe("CableTypeModal", () => {
  it("defaults to distribution but offers every fiber type, including the capacity-ruled cascades", async () => {
    render(<CableTypeModal open onCancel={() => {}} onSubmit={() => {}} />);

    expect(screen.getByText("Distribusi")).toBeInTheDocument();

    await userEvent.click(screen.getByRole("combobox"));

    expect(await screen.findByTitle("ODP ke ODP")).toBeInTheDocument();
    expect(screen.getByTitle("ODC ke ODC")).toBeInTheDocument();
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
