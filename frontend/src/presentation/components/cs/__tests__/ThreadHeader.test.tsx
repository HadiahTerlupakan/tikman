import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
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

describe("ThreadHeader on a narrow screen", () => {
  // Both controls exist only where the panes take turns. On a desktop the
  // three columns are all visible at once, so there is nothing to go back to
  // and nothing to open.
  it("offers neither a way back nor a way into the customer by default", () => {
    render(<ThreadHeader conversation={conversation} isHolder />);

    expect(screen.queryByRole("button", { name: /kembali/i })).toBeNull();
    expect(screen.queryByRole("button", { name: /Budi/ })).toBeNull();
  });

  it("goes back to the list when asked to", async () => {
    const onBack = vi.fn();
    render(
      <ThreadHeader conversation={conversation} isHolder onBack={onBack} />,
    );

    await userEvent.click(screen.getByRole("button", { name: /kembali/i }));

    expect(onBack).toHaveBeenCalledOnce();
  });

  // WhatsApp opens contact info by tapping the name at the top, so that is
  // where a CS will reach for it.
  it("opens the customer from the name in the header", async () => {
    const onOpenCustomer = vi.fn();
    render(
      <ThreadHeader
        conversation={conversation}
        isHolder
        onOpenCustomer={onOpenCustomer}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: /Budi/ }));

    expect(onOpenCustomer).toHaveBeenCalledOnce();
  });
});
