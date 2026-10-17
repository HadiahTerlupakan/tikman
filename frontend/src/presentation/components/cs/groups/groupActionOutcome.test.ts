import { describe, expect, it } from "vitest";
import type { WaGroupAction } from "@/domain/entities";
import { describeOutcome } from "./groupActionOutcome";

function action(over: Partial<WaGroupAction>): WaGroupAction {
  return {
    id: "a1",
    waAccountId: "acc1",
    action: "add",
    status: "done",
    result: "",
    failReason: "",
    createdAt: "2026-10-01T00:00:00Z",
    ...over,
  };
}

describe("describeOutcome", () => {
  it("reports the server's reason when an action failed", () => {
    expect(
      describeOutcome(action({ status: "failed", failReason: "bukan admin" })),
    ).toEqual({ level: "error", text: "bukan admin" });
  });

  // WhatsApp answers per number, so "done" can still have skipped someone.
  it("warns with the per-number notes when an add was only partly done", () => {
    const outcome = describeOutcome(
      action({ result: "62812…: sudah anggota" }),
    );
    expect(outcome.level).toBe("warning");
    expect(outcome.text).toContain("62812…: sudah anggota");
  });

  it("hands back the link for an invite link request", () => {
    const outcome = describeOutcome(
      action({
        action: "reset_invite_link",
        result: "https://chat.whatsapp.com/abc",
      }),
    );
    expect(outcome.level).toBe("success");
    expect(outcome.inviteLink).toBe("https://chat.whatsapp.com/abc");
  });
});
