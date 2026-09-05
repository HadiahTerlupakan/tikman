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

describe("ThreadHeader when the row is narrow", () => {
  const heldByOther = {
    ...conversation,
    customerName: "Muharam Nurwahid Sanjaya",
    customerPhone: "6282126568833",
    assignedUserId: "someone-else",
  };

  // At 375px the row had 318px for a back button, an avatar, the identity,
  // a "Dipegang <name>" tag and a clear button. Only the identity flexes, so
  // it was squeezed to 19px and antd's word-break shattered the number into a
  // 141px-tall column of two-digit fragments. The tag is what gives way: the
  // composer directly below already says "Dipegang <name> — ambil alih dulu
  // untuk membalas" whenever the reader is not the holder.
  it("drops the holder tag, which the composer below already states", () => {
    render(
      <ThreadHeader
        conversation={heldByOther}
        holderName="Fayadh"
        isHolder={false}
        onBack={() => {}}
      />,
    );

    expect(screen.queryByText(/Dipegang/)).toBeNull();
    expect(screen.getByText("6282126568833")).toBeInTheDocument();
  });

  it("keeps the tag where all three columns are visible", () => {
    render(
      <ThreadHeader
        conversation={heldByOther}
        holderName="Fayadh"
        isHolder={false}
      />,
    );

    expect(screen.getByText(/Dipegang Fayadh/)).toBeInTheDocument();
  });

  // jsdom lays nothing out, so the guard itself is what can be asserted: a
  // number that may not wrap cannot be broken into a column however narrow
  // its box gets.
  it("never lets the number wrap, at any width", () => {
    render(
      <ThreadHeader
        conversation={heldByOther}
        holderName="Fayadh"
        isHolder={false}
        onBack={() => {}}
      />,
    );

    expect(screen.getByText("6282126568833")).toHaveStyle({
      whiteSpace: "nowrap",
    });
  });
});
