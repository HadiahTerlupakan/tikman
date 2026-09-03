import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ConfirmByTypingModal } from "../ConfirmByTypingModal";

function open(props: Partial<Parameters<typeof ConfirmByTypingModal>[0]> = {}) {
  return render(
    <ConfirmByTypingModal
      open
      title="Hapus nomor CS Utama?"
      warning="Semua percakapan ikut terhapus."
      phrase="CS Utama"
      confirmText="Hapus nomor"
      onConfirm={vi.fn()}
      onClose={vi.fn()}
      {...props}
    />,
  );
}

const confirmButton = () => screen.getByRole("button", { name: "Hapus nomor" });

describe("ConfirmByTypingModal", () => {
  it("keeps the button dead until the phrase is typed exactly", async () => {
    const onConfirm = vi.fn();
    open({ onConfirm });

    expect(confirmButton()).toBeDisabled();

    await userEvent.type(screen.getByPlaceholderText("CS Utama"), "CS Utam");
    expect(confirmButton()).toBeDisabled();

    await userEvent.type(screen.getByPlaceholderText("CS Utama"), "a");
    expect(confirmButton()).toBeEnabled();

    await userEvent.click(confirmButton());
    expect(onConfirm).toHaveBeenCalledOnce();
  });

  // Typing the name of one number must not arm the confirmation for another.
  it("does not accept a phrase that only looks close", async () => {
    open();
    await userEvent.type(screen.getByPlaceholderText("CS Utama"), "cs utama");
    expect(confirmButton()).toBeDisabled();
  });

  // Reopening with the last confirmation still in the box would let the second
  // deletion through on a single click, which is the whole thing this guards.
  it("empties the box when it is opened again", async () => {
    const { rerender } = open();

    await userEvent.type(screen.getByPlaceholderText("CS Utama"), "CS Utama");
    expect(confirmButton()).toBeEnabled();

    const props = {
      title: "Hapus nomor CS Utama?",
      warning: "Semua percakapan ikut terhapus.",
      phrase: "CS Utama",
      confirmText: "Hapus nomor",
      onConfirm: vi.fn(),
      onClose: vi.fn(),
    };
    rerender(<ConfirmByTypingModal open={false} {...props} />);
    rerender(<ConfirmByTypingModal open {...props} />);

    expect(screen.getByPlaceholderText("CS Utama")).toHaveValue("");
    expect(confirmButton()).toBeDisabled();
  });
});
