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
      busy={false}
      onAdd={vi.fn()}
      onPair={vi.fn()}
      onClearMessages={vi.fn()}
      onDelete={vi.fn()}
      onClearInbox={vi.fn()}
      {...props}
    />,
  );
}

/** Opens one number's row menu. antd renders it into a portal on click, so
 * everything reached through it has to be awaited rather than read on the
 * next tick — doing otherwise made these tests pass alone and fail under a
 * full parallel run. */
async function openMenu(label: string) {
  await userEvent.click(
    screen.getByRole("button", { name: `Kelola ${label}` }),
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

  // The dropdown is per row, so the danger is deleting the number next to the
  // one the admin meant. The confirmation names it, and the phrase is that
  // name — typing it is what proves they read which row they were on.
  it("deletes the number whose own menu was opened", async () => {
    const onDelete = vi.fn();
    open({ onDelete });

    await openMenu("CS Teknis");
    await userEvent.click(
      await screen.findByRole("menuitem", { name: /hapus nomor/i }),
    );

    const box = await screen.findByPlaceholderText("CS Teknis");
    await userEvent.type(box, "CS Teknis");
    await userEvent.click(screen.getByRole("button", { name: "Hapus nomor" }));

    expect(onDelete).toHaveBeenCalledWith(
      expect.objectContaining({ id: "2", label: "CS Teknis" }),
    );
  });

  // A number carries years of threads. Nothing about it may go on one click.
  it("will not delete a number until its name is typed back", async () => {
    const onDelete = vi.fn();
    open({ onDelete });

    await openMenu("CS Utama");
    await userEvent.click(
      await screen.findByRole("menuitem", { name: /hapus nomor/i }),
    );

    expect(await screen.findByPlaceholderText("CS Utama")).toBeInTheDocument();
    expect(
      await screen.findByRole("button", { name: "Hapus nomor" }),
    ).toBeDisabled();
    expect(onDelete).not.toHaveBeenCalled();
  });

  it("empties the whole inbox only after the phrase is typed", async () => {
    const onClearInbox = vi.fn();
    open({ onClearInbox });

    await userEvent.click(
      screen.getByRole("button", { name: /bersihkan seluruh inbox/i }),
    );

    const confirm = await screen.findByRole("button", {
      name: "Bersihkan semua",
    });
    expect(confirm).toBeDisabled();

    await userEvent.type(
      screen.getByPlaceholderText("HAPUS SEMUA"),
      "HAPUS SEMUA",
    );
    await userEvent.click(confirm);
    expect(onClearInbox).toHaveBeenCalledOnce();
  });

  // Clearing keeps the number answering, so it asks plainly rather than for a
  // typed phrase — but it still must not fire on the first click.
  it("clears one number's messages after a plain confirmation", async () => {
    const onClearMessages = vi.fn();
    open({ onClearMessages });

    await openMenu("CS Utama");
    await userEvent.click(
      await screen.findByRole("menuitem", { name: /bersihkan pesan/i }),
    );
    expect(onClearMessages).not.toHaveBeenCalled();

    await userEvent.click(
      await screen.findByRole("button", { name: "Bersihkan" }),
    );
    expect(onClearMessages).toHaveBeenCalledWith(
      expect.objectContaining({ id: "1", label: "CS Utama" }),
    );
  });
});
