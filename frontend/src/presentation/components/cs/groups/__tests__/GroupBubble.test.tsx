import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { WaGroupMessage } from "@/domain/entities";
import { GroupThread } from "../GroupThread";
import type { GroupMessageHandlers } from "../groupMessageActions";
import { waGroup, waGroupMessage } from "./fixtures";

function handlers(): GroupMessageHandlers {
  return {
    onReply: vi.fn(),
    onReact: vi.fn(),
    onForward: vi.fn(),
    onStar: vi.fn(),
    onInfo: vi.fn(),
    onEdit: vi.fn(),
    onRevoke: vi.fn(),
    onReplyPrivate: vi.fn(),
    onVote: vi.fn(),
  };
}

function renderThread(
  messages: WaGroupMessage[],
  over: { isAdmin?: boolean; groupIsAdmin?: boolean } = {},
  h = handlers(),
) {
  render(
    <GroupThread
      messages={messages}
      onRetry={vi.fn()}
      context={{
        group: waGroup({ isAdmin: over.groupIsAdmin ?? false }),
        isAdmin: over.isAdmin ?? false,
        canSend: true,
      }}
      handlers={h}
    />,
  );
  return h;
}

async function openMenu() {
  await userEvent.click(screen.getByRole("button", { name: "Menu pesan" }));
  return (await screen.findAllByRole("menuitem")).map((i) => i.textContent);
}

/** A moment ago, so our own text is inside the edit window. */
const justNow = () => new Date(Date.now() - 60_000).toISOString();

describe("GroupThread bubbles", () => {
  it("draws a notice as a centred line with no sender and no menu", () => {
    renderThread([
      waGroupMessage({ kind: "system", body: "Budi menambahkan Rina" }),
    ]);

    expect(screen.getByRole("note")).toHaveTextContent("Budi menambahkan Rina");
    expect(screen.queryByText("Budi")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Menu pesan" })).toBeNull();
  });

  it("replaces a deleted message with the placeholder and offers nothing", () => {
    renderThread([
      waGroupMessage({
        deleted: true,
        body: "rahasia",
        reactions: [
          {
            emoji: "👍",
            count: 1,
            reactors: [{ jid: "x", name: "Ani", isSelf: false }],
          },
        ],
      }),
    ]);

    expect(screen.getByText("Pesan ini telah dihapus")).toBeInTheDocument();
    expect(screen.queryByText("rahasia")).not.toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /👍/ })).toBeNull();
    expect(screen.queryByRole("button", { name: "Menu pesan" })).toBeNull();
  });

  it("sends the whole new selection when a poll option is picked", async () => {
    const poll = waGroupMessage({
      kind: "poll",
      body: "Polling: Kapan cek ODP?",
      extra: {
        question: "Kapan cek ODP?",
        selectableCount: 0,
        options: [
          {
            name: "Pagi",
            voters: [{ jid: "me", name: "Nomor ini", isSelf: true }],
          },
          { name: "Siang", voters: [] },
        ],
      },
    });
    const h = renderThread([poll]);

    expect(screen.getByText("Kapan cek ODP?")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("checkbox", { name: "Siang" }));

    expect(h.onVote).toHaveBeenCalledWith(poll, ["Pagi", "Siang"]);
  });

  it("offers Edit, Info pesan and Hapus untuk semua on our own recent text", async () => {
    renderThread([
      waGroupMessage({ direction: "out", waTimestamp: justNow() }),
    ]);
    expect(await openMenu()).toEqual(
      expect.arrayContaining(["Edit", "Info pesan", "Hapus untuk semua"]),
    );
  });

  it("offers neither Edit nor Hapus untuk semua on a member's message to a non-admin", async () => {
    renderThread([waGroupMessage({ waTimestamp: justNow() })]);

    const items = await openMenu();
    expect(items).toEqual(expect.arrayContaining(["Balas", "Balas pribadi"]));
    expect(items).not.toContain("Edit");
    expect(items).not.toContain("Hapus untuk semua");
  });

  it("lets an admin delete a member's message where our number is group admin", async () => {
    renderThread([waGroupMessage({ waTimestamp: justNow() })], {
      isAdmin: true,
      groupIsAdmin: true,
    });

    expect(await openMenu()).toContain("Hapus untuk semua");
  });

  it("highlights a mention and toggles our reaction from its chip", async () => {
    const message = waGroupMessage({
      body: "@Ani tolong cek",
      mentionNames: ["Ani"],
      reactions: [
        {
          emoji: "👍",
          count: 1,
          reactors: [{ jid: "me", name: "Nomor ini", isSelf: true }],
        },
      ],
    });
    const h = renderThread([message]);

    expect(screen.getByText("@Ani").tagName).toBe("SPAN");
    await userEvent.click(screen.getByRole("button", { name: "👍 1" }));

    expect(h.onReact).toHaveBeenCalledWith(message, "");
  });
});
