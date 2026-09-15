import { describe, expect, it } from "vitest";
import { mayReply } from "../holding";
import type { CsConversation } from "@/domain/entities";

const thread = (overrides: Partial<CsConversation>) =>
  ({
    id: "c1",
    customerPhone: "628111222333",
    status: "open",
    lastMessageAt: "2026-09-15T01:00:00Z",
    unreadCount: 0,
    hasAvatar: false,
    ...overrides,
  }) as CsConversation;

describe("who may reply in a thread", () => {
  it("lets the holder reply", () => {
    expect(mayReply(thread({ assignedUserId: "me" }), "me")).toBe(true);
  });

  // The first reply is what makes a CS the holder, so nobody holding it must
  // not stop anyone from sending that reply.
  it("lets anyone reply to a thread nobody holds", () => {
    expect(mayReply(thread({ status: "unassigned" }), "me")).toBe(true);
  });

  it("keeps everyone else out of a thread someone is serving", () => {
    expect(mayReply(thread({ assignedUserId: "someone-else" }), "me")).toBe(
      false,
    );
  });

  // A finished thread is picked back up by taking it over, not by a reply.
  it("keeps a finished thread with its holder", () => {
    expect(
      mayReply(
        thread({ status: "closed", assignedUserId: "someone-else" }),
        "me",
      ),
    ).toBe(false);
  });
});
