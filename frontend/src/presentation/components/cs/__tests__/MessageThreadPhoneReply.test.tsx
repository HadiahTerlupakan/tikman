import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { MessageThread } from "../MessageThread";
import type { CsMessage } from "@/domain/entities";

function reply(overrides: Partial<CsMessage> = {}): CsMessage {
  return {
    id: "m1",
    conversationId: "c1",
    direction: "out",
    kind: "text",
    body: "sudah kami cek",
    status: "sent",
    waTimestamp: "2026-09-15T10:00:00Z",
    ...overrides,
  };
}

function draw(message: CsMessage) {
  return render(<MessageThread messages={[message]} onRetry={vi.fn()} />);
}

describe("a reply typed on the phone", () => {
  // Without the label a CS reads an answer nobody in TikMan wrote and asks
  // around for who sent it.
  it("is labelled as sent from the phone", () => {
    draw(reply());
    expect(screen.getByText("dari HP")).toBeInTheDocument();
  });

  it("leaves a reply sent from TikMan unlabelled", () => {
    draw(reply({ senderUserId: "u1" }));
    expect(screen.queryByText("dari HP")).not.toBeInTheDocument();
  });

  it("never labels the customer's own message", () => {
    draw(reply({ direction: "in", status: "delivered" }));
    expect(screen.queryByText("dari HP")).not.toBeInTheDocument();
  });
});
