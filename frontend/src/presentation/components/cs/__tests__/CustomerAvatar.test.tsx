import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { CustomerAvatar } from "../CustomerAvatar";
import type { CsConversation } from "@/domain/entities";

function conversation(over: Partial<CsConversation> = {}): CsConversation {
  return {
    id: "c1",
    customerPhone: "628111222333",
    customerName: "Budi",
    status: "open",
    lastMessageAt: new Date().toISOString(),
    unreadCount: 0,
    hasAvatar: false,
    ...over,
  } as CsConversation;
}

describe("CustomerAvatar", () => {
  it("shows the customer's photo when there is one", () => {
    render(
      <CustomerAvatar
        conversation={conversation({ hasAvatar: true })}
        size={42}
      />,
    );

    const img = screen.getByRole("img");
    expect(img).toHaveAttribute(
      "src",
      expect.stringContaining("/cs/conversations/c1/avatar"),
    );
  });

  // Most customers hide their photo from a number outside their contacts.
  // Asking anyway would put one 404 per row on every refresh of the inbox.
  it("asks for nothing when the customer has no photo", () => {
    const { container } = render(
      <CustomerAvatar conversation={conversation()} size={42} />,
    );

    expect(container.querySelector("img")).toBeNull();
    expect(container.querySelector(".anticon-user")).not.toBeNull();
  });
});
