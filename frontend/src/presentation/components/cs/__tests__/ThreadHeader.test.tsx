import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ThreadHeader } from "../ThreadHeader";

const conversation = {
  id: "c1",
  customerPhone: "628111222333",
  customerName: "Budi",
  assignedUserId: "me",
  status: "open" as const,
  lastMessageAt: new Date().toISOString(),
  unreadCount: 0,
  hasAvatar: false,
};

describe("ThreadHeader", () => {
  it("shows the customer's number when nobody is writing", () => {
    render(<ThreadHeader conversation={conversation} isHolder />);

    expect(screen.getByText("628111222333")).toBeInTheDocument();
    expect(screen.queryByText(/sedang mengetik/i)).toBeNull();
  });

  // A CS about to answer needs to know the customer is still adding to what
  // they asked — the number is one click away and does not change.
  it("says so while the customer is writing", () => {
    render(<ThreadHeader conversation={conversation} isHolder typing />);

    expect(screen.getByText(/sedang mengetik/i)).toBeInTheDocument();
    expect(screen.queryByText("628111222333")).toBeNull();
  });
});
