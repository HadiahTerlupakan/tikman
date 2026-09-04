import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { ConversationList } from "../ConversationList";

const rows = [
  {
    id: "c1",
    customerPhone: "628111222333",
    customerName: "Budi",
    status: "unassigned" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 2,
    hasAvatar: false,
    lastMessage: {
      body: "internet saya mati sejak pagi",
      kind: "text" as const,
      direction: "in" as const,
      at: new Date().toISOString(),
    },
  },
  {
    id: "c2",
    customerPhone: "628222333444",
    customerName: "Siti",
    assignedUserId: "kolega",
    status: "open" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 0,
    hasAvatar: false,
    lastMessage: {
      body: "",
      kind: "image" as const,
      direction: "in" as const,
      at: new Date().toISOString(),
    },
  },
  {
    id: "c3",
    customerPhone: "628333444555",
    customerName: "Rina",
    assignedUserId: "me",
    status: "open" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 0,
    hasAvatar: false,
    lastMessage: {
      body: "sudah kami cek ya",
      kind: "text" as const,
      direction: "out" as const,
      at: new Date().toISOString(),
    },
  },
];

function renderList() {
  render(
    <ConversationList
      conversations={rows}
      selectedId="c1"
      holderNames={{ me: "Saya", kolega: "Budi CS" }}
      currentUserId="me"
      onSelect={vi.fn()}
    />,
  );
}

describe("ConversationList", () => {
  it("marks a thread nobody holds", () => {
    renderList();

    expect(screen.getByText(/belum dipegang/i)).toBeInTheDocument();
  });

  // A CS scanning the list should see at a glance which threads are theirs.
  // Their own username would say the same thing, but only after they read and
  // recognise it — "Anda" is the fact itself.
  it("says a thread is yours rather than naming you", () => {
    renderList();

    expect(screen.getByText("Anda")).toBeInTheDocument();
    expect(screen.queryByText("Saya")).toBeNull();
  });

  it("names the colleague holding someone else's thread", () => {
    renderList();

    expect(screen.getByText("Budi CS")).toBeInTheDocument();
  });

  it("shows how many messages are unread", () => {
    renderList();

    expect(screen.getByText("2")).toBeInTheDocument();
  });

  // A column of names says nothing about which thread needs answering first.
  it("previews what was last said", () => {
    renderList();

    expect(
      screen.getByText(/internet saya mati sejak pagi/i),
    ).toBeInTheDocument();
  });

  // Media carries no body, so a preview built from the body alone would leave
  // a blank line where a photo is the entire message.
  it("names a photo instead of previewing its empty body", () => {
    renderList();

    expect(screen.getByText("Foto")).toBeInTheDocument();
  });

  // Without this a CS cannot tell an unanswered customer from one they already
  // replied to — the two look identical in a list of previews.
  it("marks a preview that is the team's own reply", () => {
    renderList();

    expect(screen.getByText(/Anda: sudah kami cek ya/)).toBeInTheDocument();
  });
});

describe("ConversationList typing", () => {
  // The line a customer is writing right now beats the one they finished a
  // minute ago: it is the only thing in the row that will be out of date in
  // ten seconds, and it is what tells a CS which thread to open next.
  it("shows a writing customer in place of their last message", () => {
    render(
      <ConversationList
        conversations={rows}
        typingIn={{ c1: true }}
        selectedId="c1"
        holderNames={{ me: "Saya", kolega: "Budi CS" }}
        currentUserId="me"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText(/sedang mengetik/i)).toBeInTheDocument();
    expect(screen.queryByText(/internet saya mati sejak pagi/i)).toBeNull();
  });
});
