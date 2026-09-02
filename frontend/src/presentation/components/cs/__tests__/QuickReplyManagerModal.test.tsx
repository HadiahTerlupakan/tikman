import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QuickReplyManagerModal } from "../QuickReplyManagerModal";

const create = vi.fn();
const update = vi.fn();
const remove = vi.fn();

vi.mock("@/application/hooks", () => ({
  useCreateQuickReply: () => ({ mutate: create, isPending: false }),
  useUpdateQuickReply: () => ({ mutate: update, isPending: false }),
  useDeleteQuickReply: () => ({ mutate: remove, isPending: false }),
}));

const reply = {
  id: "q1",
  title: "Gangguan massal",
  body: "Mohon maaf atas gangguannya.",
  createdBy: "u1",
  createdAt: new Date().toISOString(),
  updatedAt: new Date().toISOString(),
};

function renderModal() {
  render(
    <QuickReplyManagerModal open onClose={vi.fn()} quickReplies={[reply]} />,
  );
}

describe("QuickReplyManagerModal", () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  it("adds a template through the create mutation", async () => {
    renderModal();

    await userEvent.type(screen.getByLabelText("Judul"), "Jadwal teknisi");
    await userEvent.type(screen.getByLabelText("Isi"), "Teknisi datang besok.");
    await userEvent.click(screen.getByRole("button", { name: /tambah/i }));

    await waitFor(() =>
      expect(create).toHaveBeenCalledWith(
        { title: "Jadwal teknisi", body: "Teknisi datang besok." },
        expect.anything(),
      ),
    );
  });

  // Picking a row has to switch the same form from creating to rewriting;
  // otherwise editing a template silently produces a second copy of it.
  it("edits the row that was picked instead of creating another", async () => {
    renderModal();

    await userEvent.click(screen.getByRole("button", { name: /edit/i }));
    await userEvent.click(screen.getByRole("button", { name: /simpan/i }));

    await waitFor(() =>
      expect(update).toHaveBeenCalledWith(
        { id: "q1", title: reply.title, body: reply.body },
        expect.anything(),
      ),
    );
    expect(create).not.toHaveBeenCalled();
  });
});
