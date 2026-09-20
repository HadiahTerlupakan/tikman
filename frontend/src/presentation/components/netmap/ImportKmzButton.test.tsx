import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ImportKmzButton } from "./ImportKmzButton";

vi.mock("@/application/hooks", () => ({
  usePreviewImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
  useCommitImport: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

describe("ImportKmzButton", () => {
  it("keeps the modal closed until the button is pressed", () => {
    render(<ImportKmzButton />);

    expect(screen.queryByText("Impor Peta dari KMZ")).not.toBeInTheDocument();
  });

  it("opens the import modal when pressed", async () => {
    render(<ImportKmzButton />);

    // A regex, not an exact string: antd's icon sets its own aria-label
    // ("import"), which the button's computed accessible name includes
    // ahead of its visible text - the same reason MapToolbar's own tests
    // match "Unduh KMZ" the same way.
    await userEvent.click(screen.getByRole("button", { name: /Impor KMZ/ }));

    expect(screen.getByText("Impor Peta dari KMZ")).toBeInTheDocument();
  });
});
