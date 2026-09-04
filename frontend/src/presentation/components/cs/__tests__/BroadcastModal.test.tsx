import { describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BroadcastModal } from "../BroadcastModal";
import type { BroadcastPost, WaChannel } from "@/domain/entities";

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

const basePost: BroadcastPost = {
  id: "p1",
  waAccountId: "a1",
  destination: "channel",
  destinationJid: "120363000000000001@newsletter",
  senderUserId: "u1",
  kind: "text",
  body: "Ada pemeliharaan malam ini",
  status: "sent",
  createdAt: "2026-09-04T01:00:00Z",
};

/** MIME alone is what the modal has to go on — a real WhatsApp attachment
 * carries no other hint of what kind of file it is. */
function fileFor(kind: "document" | "image" | "video"): File {
  const type =
    kind === "image"
      ? "image/png"
      : kind === "video"
        ? "video/mp4"
        : "application/pdf";
  return new File(["x"], `attachment.${kind}`, { type });
}

function open(
  props: Partial<Parameters<typeof BroadcastModal>[0]> & {
    attachedKind?: "document" | "image" | "video";
  } = {},
) {
  const { attachedKind, ...rest } = props;
  const utils = render(
    <BroadcastModal
      open
      channels={channels}
      statusAccountIds={["a1"]}
      accountLabels={{ a1: "CS Utama" }}
      channelNames={{ "120363000000000001@newsletter": "Info Gangguan" }}
      senderNames={{ u1: "rina" }}
      posts={[]}
      loadingPosts={false}
      refreshing={false}
      sending={false}
      onSelectChannel={vi.fn()}
      onRefresh={vi.fn()}
      onSend={vi.fn().mockResolvedValue(true)}
      onClose={vi.fn()}
      {...rest}
    />,
  );
  if (attachedKind) {
    // fireEvent, not userEvent: userEvent.upload returns a promise, and every
    // test here relies on the attachment already being in place by the time
    // its synchronous assertions run. Queried from document.body, not the
    // render container: antd's Modal portals its content there.
    const input = document.body.querySelector(
      'input[type="file"]',
    ) as HTMLInputElement;
    fireEvent.change(input, { target: { files: [fileFor(attachedKind)] } });
  }
  return utils;
}

const sendButton = () =>
  screen.getByRole("button", { name: "Kirim Pembaruan" });

describe("BroadcastModal", () => {
  // An announcement reaches every subscriber or viewer and cannot be
  // withdrawn. Sending before a destination is chosen would mean sending to
  // whichever one happened to be first.
  it("keeps the send button dead while no channel is chosen", async () => {
    open();
    expect(sendButton()).toBeDisabled();

    await userEvent.type(
      screen.getByPlaceholderText(/tulis pengumuman/i),
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
      screen.getByPlaceholderText(/tulis pengumuman/i),
      "Ada pemeliharaan",
    );

    expect(sendButton()).toBeEnabled();
  });

  // Sending only queues; the outcome arrives seconds later. The history is the
  // only place a failure is ever visible, so the reason has to be on screen.
  it("shows why an update failed", () => {
    const failed = [
      {
        ...basePost,
        status: "failed" as const,
        failReason: "not authorized to post",
      },
    ];
    open({ selectedChannelId: "c1", posts: failed });

    expect(screen.getByText("Gagal")).toBeInTheDocument();
    expect(screen.getByText("not authorized to post")).toBeInTheDocument();
  });

  // Admin, CS and Technician can all broadcast irrevocably to every
  // subscriber or viewer. That was accepted on the strength of the history
  // being honest about who did it, so the name is the accountability half of
  // the bargain.
  it("names who posted each update", () => {
    open({ selectedChannelId: "c1", posts: [basePost] });

    expect(screen.getByText("rina")).toBeInTheDocument();
  });

  // A poster whose account has since been removed still has to render as
  // something a reader can understand, not as a raw id.
  it("says so plainly when the poster is no longer a user", () => {
    open({
      selectedChannelId: "c1",
      posts: [{ ...basePost, senderUserId: "u-gone" }],
    });

    expect(screen.getByText(/tak dikenal/i)).toBeInTheDocument();
    expect(screen.queryByText("u-gone")).toBeNull();
  });

  // A number that admins no channel cannot be helped by the Saluran option,
  // and the empty state has to say why — but Status does not depend on a
  // channel existing at all, so it must stay usable.
  it("disables the Saluran option when there are no channels, but Status stays usable", () => {
    open({ channels: [] });

    expect(screen.getByRole("checkbox", { name: "Saluran" })).toBeDisabled();
    expect(
      screen.getByText(/harus sudah menjadi admin sebuah saluran/i),
    ).toBeInTheDocument();
    expect(screen.getByRole("checkbox", { name: /status wa/i })).toBeEnabled();
  });

  // A document cannot be posted as a status. Disabling the checkbox is what
  // makes that visible before the sender commits to it, rather than seconds
  // later in the history.
  it("refuses a status destination once a document is attached", async () => {
    open({ selectedChannelId: "c1", attachedKind: "document" });

    expect(screen.getByRole("checkbox", { name: /status wa/i })).toBeDisabled();
    expect(
      screen.getByText(/status hanya menerima teks, gambar, dan video/i),
    ).toBeInTheDocument();
  });

  // One action can now reach two places, so the history has to say which is
  // which — otherwise a partial failure is unreadable.
  it("labels each history row with where it went", () => {
    open({
      posts: [
        {
          ...basePost,
          id: "p1",
          destination: "channel",
          destinationJid: "120363000000000001@newsletter",
        },
        { ...basePost, id: "p2", destination: "status" },
      ],
    });

    expect(screen.getByText(/Saluran · Info Gangguan/)).toBeInTheDocument();
    expect(screen.getByText(/Status · CS Utama/)).toBeInTheDocument();
  });

  // A status is gone after 24 hours. Dropping the Terkirim tag would erase that
  // it once succeeded; dropping the note would leave old rows reading as live.
  it("marks a status older than a day as expired while keeping its sent tag", () => {
    const twoDaysAgo = new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString();
    open({
      posts: [
        {
          ...basePost,
          id: "p3",
          destination: "status",
          status: "sent",
          sentAt: twoDaysAgo,
        },
      ],
    });

    expect(screen.getByText("Terkirim")).toBeInTheDocument();
    expect(screen.getByText(/sudah kedaluwarsa/i)).toBeInTheDocument();
  });

  // A channel post does not expire, so the note must not appear on one.
  it("does not mark a channel post as expired", () => {
    const twoDaysAgo = new Date(Date.now() - 48 * 60 * 60 * 1000).toISOString();
    open({
      posts: [
        {
          ...basePost,
          id: "p4",
          destination: "channel",
          destinationJid: "120363000000000001@newsletter",
          status: "sent",
          sentAt: twoDaysAgo,
        },
      ],
    });

    expect(screen.queryByText(/sudah kedaluwarsa/i)).toBeNull();
  });
});
