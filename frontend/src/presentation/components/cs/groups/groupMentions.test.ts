import { describe, expect, it } from "vitest";
import type { WaGroupParticipant } from "@/domain/entities";
import {
  MAX_MENTIONS,
  highlightedNames,
  mentionLimitError,
  mentionOptions,
  mentionsEveryone,
  splitMentions,
  toWhatsAppMentions,
} from "./groupMentions";

const member = (
  over: Partial<WaGroupParticipant> = {},
): WaGroupParticipant => ({
  jid: "55511122233344@lid",
  phone: "",
  name: "Pak Joko",
  pushName: "",
  hasAvatar: false,
  role: "member",
  isSelf: false,
  ...over,
});

describe("toWhatsAppMentions", () => {
  it("writes the member's address into the text and lists their JID", () => {
    expect(toWhatsAppMentions("@Pak Joko tolong cek", [member()])).toEqual({
      body: "@55511122233344 tolong cek",
      mentions: ["55511122233344@lid"],
    });
  });

  // "@Budi" must not swallow the start of "@Budi Santoso".
  it("matches the longer of two names that share a start", () => {
    const budi = member({ jid: "628111@s.whatsapp.net", name: "Budi" });
    const santoso = member({
      jid: "628222@s.whatsapp.net",
      name: "Budi Santoso",
    });

    const out = toWhatsAppMentions("@Budi Santoso dan @Budi", [budi, santoso]);

    expect(out.body).toBe("@628222 dan @628111");
    expect(out.mentions).toEqual([
      "628222@s.whatsapp.net",
      "628111@s.whatsapp.net",
    ]);
  });

  it("leaves a message with no mention as it was", () => {
    expect(toWhatsAppMentions("halo semua", [member()])).toEqual({
      body: "halo semua",
      mentions: [],
    });
  });
});

describe("mentionOptions", () => {
  it("offers every member except our own number", () => {
    const options = mentionOptions([
      member(),
      member({ jid: "628999@s.whatsapp.net", isSelf: true, name: "" }),
    ]);
    expect(options.map((o) => o.value)).toEqual(["semua", "Pak Joko"]);
  });

  it("offers everyone first, labelled as such", () => {
    expect(mentionOptions([member()])[0]).toEqual({
      key: "@semua",
      value: "semua",
      label: "semua — tag semua anggota",
    });
  });
});

describe("@semua", () => {
  const joko = member();
  const budi = member({ jid: "628111@s.whatsapp.net", name: "Budi" });
  const self = member({ jid: "628999@s.whatsapp.net", isSelf: true });

  it("is the whole token only", () => {
    expect(mentionsEveryone("@semua")).toBe(true);
    expect(mentionsEveryone("rapat jam 9 @semua.")).toBe(true);
    expect(mentionsEveryone("@semua, tolong cek")).toBe(true);
    expect(mentionsEveryone("@semuanya hadir")).toBe(false);
    expect(mentionsEveryone("halo semua")).toBe(false);
  });

  it("tags every member but our own number and keeps the text as typed", () => {
    expect(
      toWhatsAppMentions("@semua rapat jam 9", [joko, self, budi]),
    ).toEqual({
      body: "@semua rapat jam 9",
      mentions: ["55511122233344@lid", "628111@s.whatsapp.net"],
    });
  });

  it("lists a member also named in the message once", () => {
    const out = toWhatsAppMentions("@semua, @Budi tolong cek", [joko, budi]);

    expect(out.body).toBe("@semua, @628111 tolong cek");
    expect(out.mentions).toEqual([
      "628111@s.whatsapp.net",
      "55511122233344@lid",
    ]);
  });

  it("does not take @semuanya for everyone", () => {
    expect(toWhatsAppMentions("@semuanya hadir", [joko, budi])).toEqual({
      body: "@semuanya hadir",
      mentions: [],
    });
  });

  // The picker's "@semua" and a member named "semua" read the same.
  it("means everyone even when a member is named semua", () => {
    const named = member({ jid: "628333@s.whatsapp.net", name: "semua" });

    expect(toWhatsAppMentions("@semua cek", [joko, named])).toEqual({
      body: "@semua cek",
      mentions: ["55511122233344@lid", "628333@s.whatsapp.net"],
    });
  });

  it("is not rewritten by a shorter name it starts with", () => {
    const s = member({ jid: "628444@s.whatsapp.net", name: "s" });

    expect(toWhatsAppMentions("@semua cek", [joko, s]).body).toBe("@semua cek");
  });

  it("refuses more mentions than the API takes", () => {
    const many = Array.from({ length: MAX_MENTIONS + 1 }, (_, i) =>
      member({ jid: `62${i}@s.whatsapp.net`, name: `Anggota ${i}` }),
    );
    const { mentions } = toWhatsAppMentions("@semua", many);

    expect(mentions).toHaveLength(MAX_MENTIONS + 1);
    expect(mentionLimitError(mentions)).toMatch(/maks 1024 anggota/);
    expect(mentionLimitError(mentions.slice(1))).toBeNull();
  });

  it("is highlighted only in a message that tagged someone", () => {
    expect(
      splitMentions(
        "@semua cek",
        highlightedNames("@semua cek", ["Budi"], true),
      ),
    ).toEqual([
      { text: "@semua", mention: true },
      { text: " cek", mention: false },
    ]);
    expect(highlightedNames("@semua cek", [], false)).toEqual([]);
    expect(highlightedNames("@semuanya cek", ["Budi"], true)).toEqual(["Budi"]);
  });

  // Members with no name yet are notified but missing from mentionNames.
  it("is highlighted when every tagged member is unnamed", () => {
    expect(
      splitMentions("@semua cek", highlightedNames("@semua cek", [], true)),
    ).toEqual([
      { text: "@semua", mention: true },
      { text: " cek", mention: false },
    ]);
  });

  it("does not highlight @semuanya beside a real @semua", () => {
    const body = "@semua x @semuanya";
    expect(splitMentions(body, highlightedNames(body, ["Budi"], true))).toEqual(
      [
        { text: "@semua", mention: true },
        { text: " x @semuanya", mention: false },
      ],
    );
  });
});

describe("splitMentions", () => {
  it("picks out the tagged names, longest first", () => {
    expect(
      splitMentions("@Budi Santoso dan @Budi cek ODP", [
        "Budi",
        "Budi Santoso",
      ]),
    ).toEqual([
      { text: "@Budi Santoso", mention: true },
      { text: " dan ", mention: false },
      { text: "@Budi", mention: true },
      { text: " cek ODP", mention: false },
    ]);
  });

  // An "@" typed by hand notified nobody, so it is not drawn as a mention.
  it("leaves an @ that tags nobody as text", () => {
    expect(splitMentions("kirim ke rohadi@mail.id", ["Budi"])).toEqual([
      { text: "kirim ke rohadi@mail.id", mention: false },
    ]);
  });
});
