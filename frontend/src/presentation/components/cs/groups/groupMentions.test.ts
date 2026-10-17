import { describe, expect, it } from "vitest";
import type { WaGroupParticipant } from "@/domain/entities";
import {
  mentionOptions,
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
    expect(options.map((o) => o.value)).toEqual(["Pak Joko"]);
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
