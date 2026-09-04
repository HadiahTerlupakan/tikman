import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChannelBroadcastModal } from "../ChannelBroadcastModal";
import type { ChannelPost, WaChannel } from "@/domain/entities";

const channels: WaChannel[] = [
  {
    id: "c1",
    waAccountId: "a1",
    jid: "120363000000000001@newsletter",
    name: "Info Gangguan",
    role: "owner",
    subscriberCount: 240,
    syncedAt: "2026-09-04T00:00:00Z",
  },
];

function open(
  props: Partial<Parameters<typeof ChannelBroadcastModal>[0]> = {},
) {
  return render(
    <ChannelBroadcastModal
      open
      channels={channels}
      accountLabels={{ a1: "CS Utama" }}
      senderNames={{ u1: "rina" }}
      posts={[]}
      loadingPosts={false}
      refreshing={false}
      sending={false}
      onSelectChannel={vi.fn()}
      onRefresh={vi.fn()}
      onSend={vi.fn().mockResolvedValue(true)}
      onClose={vi.fn()}
      {...props}
    />,
  );
}

function post(overrides: Partial<ChannelPost>): ChannelPost {
  return {
    id: "p1",
    waAccountId: "a1",
    channelJid: "120363000000000001@newsletter",
    senderUserId: "u1",
    kind: "text",
    body: "Ada pemeliharaan malam ini",
    status: "sent",
    createdAt: "2026-09-04T01:00:00Z",
    ...overrides,
  };
}

const sendButton = () =>
  screen.getByRole("button", { name: "Kirim Pembaruan" });

describe("ChannelBroadcastModal", () => {
  // An update reaches every subscriber and cannot be withdrawn. Sending before
  // a channel is chosen would mean sending to whichever one happened to be
  // first.
  it("keeps the send button dead while no channel is chosen", async () => {
    open();
    expect(sendButton()).toBeDisabled();

    await userEvent.type(
      screen.getByPlaceholderText(/tulis pembaruan/i),
      "Ada pemeliharaan",
    );

    expect(sendButton()).toBeDisabled();
  });

  it("keeps the send button dead while there is nothing to say", () => {
    open({ selectedChannelId: "c1" });

    expect(sendButton()).toBeDisabled();
  });

  it("arms the send button once both are present", async () => {
    open({ selectedChannelId: "c1" });

    await userEvent.type(
      screen.getByPlaceholderText(/tulis pembaruan/i),
      "Ada pemeliharaan",
    );

    expect(sendButton()).toBeEnabled();
  });

  // Sending only queues; the outcome arrives seconds later. The history is the
  // only place a failure is ever visible, so the reason has to be on screen.
  it("shows why an update failed", () => {
    const failed = [
      post({ status: "failed", failReason: "not authorized to post" }),
    ];
    open({ selectedChannelId: "c1", posts: failed });

    expect(screen.getByText("Gagal")).toBeInTheDocument();
    expect(screen.getByText("not authorized to post")).toBeInTheDocument();
  });

  // Admin, CS and Technician can all broadcast irrevocably to every
  // subscriber. That was accepted on the strength of the history being honest
  // about who did it, so the name is the accountability half of the bargain.
  it("names who posted each update", () => {
    open({ selectedChannelId: "c1", posts: [post({})] });

    expect(screen.getByText("rina")).toBeInTheDocument();
  });

  // A poster whose account has since been removed still has to render as
  // something a reader can understand, not as a raw id.
  it("says so plainly when the poster is no longer a user", () => {
    open({
      selectedChannelId: "c1",
      posts: [post({ senderUserId: "u-gone" })],
    });

    expect(screen.getByText(/tak dikenal/i)).toBeInTheDocument();
    expect(screen.queryByText("u-gone")).toBeNull();
  });

  // A number that admins no channel cannot be helped by this screen, and the
  // empty state has to say why rather than look like a loading failure.
  it("explains an empty channel list instead of showing a dead composer", () => {
    open({ channels: [] });

    expect(
      screen.getByText(/harus sudah menjadi admin sebuah saluran/i),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Kirim Pembaruan" }),
    ).toBeNull();
  });
});
