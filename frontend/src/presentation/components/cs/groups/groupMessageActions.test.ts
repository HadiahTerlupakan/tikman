import { describe, expect, it } from "vitest";
import { groupMessageActions } from "./groupMessageActions";
import { waGroup, waGroupMessage } from "./__tests__/fixtures";

const sentAt = "2026-10-17T07:05:00Z";
const now = new Date(sentAt).getTime();
const ctx = {
  group: waGroup(),
  isAdmin: false,
  userId: "u1",
  canSend: true,
  now,
};

describe("groupMessageActions", () => {
  it("offers an edit only on text the viewer wrote, inside fifteen minutes", () => {
    const ours = waGroupMessage({
      direction: "out",
      senderUserId: "u1",
      waTimestamp: sentAt,
    });
    expect(groupMessageActions(ours, ctx)).toContain("edit");
    expect(
      groupMessageActions(ours, { ...ctx, now: now + 16 * 60 * 1000 }),
    ).not.toContain("edit");
    expect(groupMessageActions({ ...ours, kind: "image" }, ctx)).not.toContain(
      "edit",
    );
    expect(groupMessageActions(waGroupMessage(), ctx)).not.toContain("edit");
  });

  // A message carries its writer's initials: a colleague changing or erasing
  // it would be speaking in their name. An admin may still delete it.
  it("leaves a colleague's message alone unless the viewer is an admin", () => {
    const colleagues = waGroupMessage({
      direction: "out",
      senderUserId: "u2",
      waTimestamp: sentAt,
    });
    const asCS = groupMessageActions(colleagues, ctx);
    expect(asCS).not.toContain("edit");
    expect(asCS).not.toContain("revoke");
    const asAdmin = groupMessageActions(colleagues, { ...ctx, isAdmin: true });
    expect(asAdmin).not.toContain("edit");
    expect(asAdmin).toContain("revoke");
  });

  // Someone else's message is WhatsApp's to refuse unless our number is a
  // group admin, and the API's unless the viewer is a TikMan admin.
  it("deletes someone else's message only for an admin in a group we administer", () => {
    const theirs = waGroupMessage();
    const adminGroup = waGroup({ isAdmin: true });
    expect(groupMessageActions(theirs, ctx)).not.toContain("revoke");
    expect(
      groupMessageActions(theirs, { ...ctx, isAdmin: true }),
    ).not.toContain("revoke");
    expect(
      groupMessageActions(theirs, { ...ctx, group: adminGroup }),
    ).not.toContain("revoke");
    expect(
      groupMessageActions(theirs, { ...ctx, isAdmin: true, group: adminGroup }),
    ).toContain("revoke");
    expect(
      groupMessageActions(
        waGroupMessage({ direction: "out", senderUserId: "u1" }),
        ctx,
      ),
    ).toContain("revoke");
  });

  it("keeps message info to ours and a private reply to theirs", () => {
    const ours = groupMessageActions(waGroupMessage({ direction: "out" }), ctx);
    const theirs = groupMessageActions(waGroupMessage(), ctx);
    expect(ours).toContain("info");
    expect(ours).not.toContain("replyPrivate");
    expect(theirs).toContain("replyPrivate");
    expect(theirs).not.toContain("info");
  });

  it("offers nothing on a notice, a deleted message or one still queued", () => {
    expect(
      groupMessageActions(waGroupMessage({ kind: "system" }), ctx),
    ).toEqual([]);
    expect(groupMessageActions(waGroupMessage({ deleted: true }), ctx)).toEqual(
      [],
    );
    expect(
      groupMessageActions(
        waGroupMessage({ direction: "out", status: "queued" }),
        ctx,
      ),
    ).toEqual([]);
  });

  it("does not forward a poll", () => {
    expect(
      groupMessageActions(waGroupMessage({ kind: "poll" }), ctx),
    ).not.toContain("forward");
  });
});
