import { describe, expect, it } from "vitest";
import { statusHasExpired } from "../BroadcastHistory";
import type { BroadcastPost } from "@/domain/entities";

const DAY_MS = 24 * 60 * 60 * 1000;
const sentAt = "2026-09-04T00:00:00.000Z";
const sentAtMs = Date.parse(sentAt);

function post(overrides: Partial<BroadcastPost> = {}): BroadcastPost {
  return {
    id: "p1",
    waAccountId: "a1",
    destination: "status",
    senderUserId: "u1",
    kind: "text",
    body: "Ada pemeliharaan malam ini",
    status: "sent",
    createdAt: sentAt,
    sentAt,
    ...overrides,
  };
}

// The 24-hour rule is calculated, not stored, so nothing but this decides
// whether a row reads as still live. Rendering only ever exercises it well
// clear of the boundary, which is where an off-by-one would hide.
describe("statusHasExpired", () => {
  it("holds a status live for its first day", () => {
    expect(statusHasExpired(post(), sentAtMs + DAY_MS - 1)).toBe(false);
  });

  // A status posted exactly 24 hours ago is still WhatsApp's to show. The
  // note belongs on the row only once that day is actually over.
  it("does not expire a status at exactly 24 hours", () => {
    expect(statusHasExpired(post(), sentAtMs + DAY_MS)).toBe(false);
  });

  it("expires a status the moment its day is past", () => {
    expect(statusHasExpired(post(), sentAtMs + DAY_MS + 1)).toBe(true);
  });

  // A channel update stays on the channel indefinitely. Ageing one out would
  // tell a reader something disappeared that is still there.
  it("never expires a channel post, however old", () => {
    const channel = post({
      destination: "channel",
      destinationJid: "120363000000000001@newsletter",
    });

    expect(statusHasExpired(channel, sentAtMs + 30 * DAY_MS)).toBe(false);
  });

  // A queued or failed status never went out, so there is nothing for a day
  // to have passed on — and sentAt is empty on exactly those rows.
  it("does not expire a status that never went out", () => {
    const queued = post({ status: "queued", sentAt: undefined });

    expect(statusHasExpired(queued, sentAtMs + 30 * DAY_MS)).toBe(false);
  });
});
