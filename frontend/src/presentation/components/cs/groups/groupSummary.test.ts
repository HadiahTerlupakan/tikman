import { describe, expect, it } from "vitest";
import type { WaGroup, WaGroupMessage } from "@/domain/entities";
import {
  groupPreview,
  parseParticipants,
  participantLabel,
  sendBlockedReason,
  senderLabel,
  typingLine,
} from "./groupSummary";

function group(over: Partial<WaGroup> = {}): WaGroup {
  return {
    id: "g1",
    waAccountId: "acc1",
    jid: "120363000000000001@g.us",
    name: "Teknisi Cariu",
    topic: "",
    ownerJid: "",
    participantCount: 4,
    isAdmin: false,
    announce: false,
    locked: false,
    left: false,
    lastMessagePreview: "",
    lastMessageKind: "",
    lastMessageSender: "",
    unreadCount: 0,
    hasAvatar: false,
    pinned: false,
    createdAt: "2026-10-01T00:00:00Z",
    updatedAt: "2026-10-01T00:00:00Z",
    ...over,
  };
}

describe("groupPreview", () => {
  it("names who spoke last before what they said", () => {
    expect(
      groupPreview(
        group({
          lastMessageKind: "text",
          lastMessagePreview: "ODP 3 sudah nyala",
          lastMessageSender: "Budi",
        }),
      ),
    ).toBe("Budi: ODP 3 sudah nyala");
  });

  // Media arrives with an empty body; a blank line would hide that a photo
  // is the whole message.
  it("labels a photo with no caption", () => {
    expect(
      groupPreview(
        group({ lastMessageKind: "image", lastMessageSender: "Ani" }),
      ),
    ).toBe("Ani: Foto");
  });

  // Our own number has a name like any member's; the list should still
  // read as "we said this".
  it("calls our own last message ours", () => {
    expect(
      groupPreview(
        group({
          lastMessageKind: "text",
          lastMessagePreview: "siap",
          lastMessageSender: "admin",
          lastMessageDirection: "out",
        }),
      ),
    ).toBe("Anda: siap");
  });

  it("says so when nothing has been said yet", () => {
    expect(groupPreview(group())).toBe("Belum ada pesan");
  });
});

describe("sendBlockedReason", () => {
  it("lets a member write to an open group", () => {
    expect(sendBlockedReason(group())).toBeNull();
  });

  it("blocks a group our number has left", () => {
    expect(sendBlockedReason(group({ left: true, isAdmin: true }))).toMatch(
      /keluar/,
    );
  });

  it("blocks an admin-only group when our number is not an admin", () => {
    expect(sendBlockedReason(group({ announce: true }))).toMatch(/admin/);
  });

  it("lets our number write to an admin-only group it administers", () => {
    expect(
      sendBlockedReason(group({ announce: true, isAdmin: true })),
    ).toBeNull();
  });
});

describe("senderLabel", () => {
  const base = {
    direction: "in",
    senderJid: "123456789@lid",
    senderPhone: "",
    senderName: "",
  } as WaGroupMessage;

  it("falls back from name to number to the bare jid", () => {
    expect(senderLabel({ ...base, senderName: "Budi" })).toBe("Budi");
    expect(senderLabel({ ...base, senderPhone: "628123" })).toBe("+628123");
    expect(senderLabel(base)).toBe("123456789");
  });

  // WhatsApp marks a name the member chose for themselves with "~", so it is
  // not mistaken for one somebody on the team saved.
  it("shows a profile name with a tilde when no saved name exists", () => {
    expect(senderLabel({ ...base, senderPushName: "Rina" })).toBe("~Rina");
    expect(
      senderLabel({ ...base, senderName: "Bu Rina", senderPushName: "Rina" }),
    ).toBe("Bu Rina");
    expect(
      participantLabel({
        jid: base.senderJid,
        phone: "628123",
        name: "",
        pushName: "Rina",
        hasAvatar: false,
        role: "member",
        isSelf: false,
      }),
    ).toBe("~Rina");
  });

  it("calls our own number Anda", () => {
    expect(senderLabel({ ...base, direction: "out", senderName: "CS" })).toBe(
      "Anda",
    );
  });
});

describe("parseParticipants", () => {
  it("splits on lines, commas and semicolons but keeps a spaced number whole", () => {
    expect(
      parseParticipants("0812 3456 7890\n+62-813-1111, 62814;0812 3456 7890"),
    ).toEqual(["081234567890", "+628131111", "62814"]);
  });

  it("returns nothing for blank input", () => {
    expect(parseParticipants(" \n , ")).toEqual([]);
  });
});

describe("typingLine", () => {
  it("names one or two writers and counts more", () => {
    expect(typingLine([])).toBeNull();
    expect(typingLine(["Budi"])).toBe("Budi sedang mengetik…");
    expect(typingLine(["Budi", "Ani"])).toBe("Budi dan Ani sedang mengetik…");
    expect(typingLine(["Budi", "Ani", "Rina"])).toBe(
      "3 orang sedang mengetik…",
    );
  });
});
