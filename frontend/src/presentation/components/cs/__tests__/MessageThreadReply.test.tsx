import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { MessageThread } from "../MessageThread";
import type { CsMessage } from "@/domain/entities";

function message(over: Partial<CsMessage> = {}): CsMessage {
  return {
    id: "m1",
    conversationId: "c1",
    direction: "in",
    kind: "text",
    body: "internet saya mati",
    status: "delivered",
    waTimestamp: "2026-09-03T07:05:00Z",
    ...over,
  } as CsMessage;
}

describe("MessageThread replies", () => {
  // A shared inbox loses track of what an answer was for: three complaints,
  // three "sudah kami cek", and nobody can tell which belongs to which.
  it("names the message a reply answers", () => {
    render(
      <MessageThread
        messages={[
          message({
            id: "m2",
            direction: "out",
            body: "sudah kami cek",
            replyToId: "m1",
            replyTo: {
              id: "m1",
              direction: "in",
              kind: "text",
              body: "internet saya mati",
            },
          }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText("internet saya mati")).toBeInTheDocument();
    expect(screen.getByText("Pelanggan")).toBeInTheDocument();
  });

  // A quoted photo has no words of its own. Left blank, the block reads as a
  // rendering fault rather than as a picture.
  it("says what a quoted attachment was when it carried no caption", () => {
    render(
      <MessageThread
        messages={[
          message({
            id: "m2",
            direction: "out",
            body: "ini modem yang mana ya",
            replyTo: {
              id: "m1",
              direction: "in",
              kind: "image",
              body: "",
            },
          }),
        ]}
        onRetry={vi.fn()}
      />,
    );

    expect(screen.getByText("Foto")).toBeInTheDocument();
  });

  it("draws no quoted block on a message that quotes nothing", () => {
    render(<MessageThread messages={[message()]} onRetry={vi.fn()} />);

    expect(screen.queryByText("Pelanggan")).toBeNull();
  });

  it("hands the whole message back when a reply is started", async () => {
    const onReply = vi.fn();
    render(
      <MessageThread
        messages={[message()]}
        onRetry={vi.fn()}
        onReply={onReply}
      />,
    );

    await userEvent.click(
      screen.getByRole("button", { name: /balas pesan ini/i }),
    );

    expect(onReply).toHaveBeenCalledWith(
      expect.objectContaining({ id: "m1", body: "internet saya mati" }),
    );
  });

  // Offering the gesture to someone who cannot send, and then refusing the
  // send, is the worse of the two failures.
  it("offers no way to reply to anyone who cannot send on the thread", () => {
    render(<MessageThread messages={[message()]} onRetry={vi.fn()} />);

    expect(
      screen.queryByRole("button", { name: /balas pesan ini/i }),
    ).toBeNull();
  });
});
