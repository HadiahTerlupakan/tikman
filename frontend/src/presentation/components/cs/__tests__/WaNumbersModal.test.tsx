import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { WaNumbersModal } from "../WaNumbersModal";
import type { WaAccount, WaAccountStatus } from "@/domain/entities";

function account(
  id: string,
  label: string,
  status: WaAccountStatus,
  jid?: string,
): WaAccount {
  return { id, label, status, jid, createdAt: "", updatedAt: "" };
}

function open(props: Partial<Parameters<typeof WaNumbersModal>[0]> = {}) {
  return render(
    <WaNumbersModal
      open
      onClose={vi.fn()}
      accounts={[
        account("1", "CS Utama", "connected", "6281399977707@s.whatsapp.net"),
        account("2", "CS Teknis", "disconnected"),
      ]}
      stream={{}}
      adding={false}
      onAdd={vi.fn()}
      onPair={vi.fn()}
      {...props}
    />,
  );
}

describe("WaNumbersModal", () => {
  it("lists every number with its state", () => {
    open();

    expect(screen.getByText("CS Utama")).toBeInTheDocument();
    expect(screen.getByText("Terhubung")).toBeInTheDocument();
    expect(screen.getByText("CS Teknis")).toBeInTheDocument();
    expect(screen.getByText("Terputus")).toBeInTheDocument();
  });

  // A number is a name until WhatsApp says otherwise. Showing a blank where
  // the number goes reads as a fault rather than as "not paired yet".
  it("says a number is not paired yet rather than showing a blank", () => {
    open();
    expect(screen.getByText("belum terpasang")).toBeInTheDocument();
  });

  it("hands the whole account back when one is chosen for pairing", async () => {
    const onPair = vi.fn();
    open({ onPair });

    await userEvent.click(screen.getByRole("button", { name: "Sambungkan" }));
    expect(onPair).toHaveBeenCalledWith(
      expect.objectContaining({ id: "2", label: "CS Teknis" }),
    );
  });

  it("adds a number by name and clears the box", async () => {
    const onAdd = vi.fn();
    open({ onAdd });

    const box = screen.getByPlaceholderText(/nama nomor baru/i);
    await userEvent.type(box, "CS Penagihan");
    await userEvent.click(screen.getByRole("button", { name: /tambah/i }));

    expect(onAdd).toHaveBeenCalledWith("CS Penagihan");
    expect(box).toHaveValue("");
  });

  // A number with no name cannot be told apart from the others in the inbox,
  // which is the one job the name has.
  it("refuses to add a number with no name", async () => {
    const onAdd = vi.fn();
    open({ onAdd });

    await userEvent.type(
      screen.getByPlaceholderText(/nama nomor baru/i),
      "   ",
    );
    await userEvent.click(screen.getByRole("button", { name: /tambah/i }));
    expect(onAdd).not.toHaveBeenCalled();
  });
});
