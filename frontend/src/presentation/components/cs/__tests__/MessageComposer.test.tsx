import { describe, expect, it, vi } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MessageComposer } from "../MessageComposer";

const conversation = {
  id: "c1",
  customerPhone: "628111222333",
  customerName: "Budi",
  status: "open" as const,
  lastMessageAt: new Date().toISOString(),
  unreadCount: 0,
};

describe("MessageComposer", () => {
  // A greyed-out button with no explanation reads as a broken page. The CS
  // needs to know it is held by someone, and that taking over is the way in.
  it("says who holds the thread instead of just disabling the button", () => {
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "someone-else" }}
        currentUserId="me"
        holderName="Budi CS"
        onSend={vi.fn()}
        onTakeOver={vi.fn()}
      />,
    );

    expect(screen.getByText(/Dipegang Budi CS/i)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /ambil alih/i })).toBeEnabled();
    expect(screen.queryByRole("button", { name: /^kirim$/i })).toBeNull();
  });

  it("lets the holder send", () => {
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "me" }}
        currentUserId="me"
        holderName="Saya"
        onSend={vi.fn()}
        onTakeOver={vi.fn()}
      />,
    );

    expect(
      screen.getByRole("button", { name: /^kirim$/i }),
    ).toBeInTheDocument();
  });

  // The composer used to clear itself before the result was known, so a send
  // refused with a 409 cost the CS the reply they had just written.
  it("keeps what was typed when the send does not go through", async () => {
    const onSend = vi.fn().mockResolvedValue(false);
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "me" }}
        currentUserId="me"
        holderName="Saya"
        onSend={onSend}
        onTakeOver={vi.fn()}
      />,
    );

    const box = screen.getByPlaceholderText(/tulis balasan/i);
    await userEvent.type(box, "halo");
    await userEvent.click(screen.getByRole("button", { name: /^kirim$/i }));

    expect(onSend).toHaveBeenCalledWith("halo");
    await waitFor(() => expect(box).toHaveValue("halo"));
  });

  it("clears the box once the reply has left", async () => {
    const onSend = vi.fn().mockResolvedValue(true);
    render(
      <MessageComposer
        conversation={{ ...conversation, assignedUserId: "me" }}
        currentUserId="me"
        holderName="Saya"
        onSend={onSend}
        onTakeOver={vi.fn()}
      />,
    );

    const box = screen.getByPlaceholderText(/tulis balasan/i);
    await userEvent.type(box, "halo");
    await userEvent.click(screen.getByRole("button", { name: /^kirim$/i }));

    await waitFor(() => expect(box).toHaveValue(""));
  });
});
