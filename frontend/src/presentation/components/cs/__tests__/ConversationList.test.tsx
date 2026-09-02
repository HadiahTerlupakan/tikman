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
  },
  {
    id: "c2",
    customerPhone: "628222333444",
    customerName: "Siti",
    assignedUserId: "me",
    status: "open" as const,
    lastMessageAt: new Date().toISOString(),
    unreadCount: 0,
  },
];

describe("ConversationList", () => {
  it("marks a thread nobody holds", () => {
    render(
      <ConversationList
        conversations={rows}
        selectedId="c1"
        holderNames={{ me: "Saya" }}
        currentUserId="me"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText(/belum dipegang/i)).toBeInTheDocument();
    expect(screen.getByText("Saya")).toBeInTheDocument();
  });

  it("shows how many messages are unread", () => {
    render(
      <ConversationList
        conversations={rows}
        selectedId="c1"
        holderNames={{ me: "Saya" }}
        currentUserId="me"
        onSelect={vi.fn()}
      />,
    );

    expect(screen.getByText("2")).toBeInTheDocument();
  });
});
