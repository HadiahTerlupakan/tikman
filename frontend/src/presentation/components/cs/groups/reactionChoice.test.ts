import { describe, expect, it } from "vitest";
import type { WaGroupReaction } from "@/domain/entities";
import { ownReaction, reactionToSend } from "./reactionChoice";

const budi = { jid: "628111@s.whatsapp.net", name: "Budi", isSelf: false };
const us = { jid: "628999@s.whatsapp.net", name: "Nomor ini", isSelf: true };

const reactions: WaGroupReaction[] = [
  { emoji: "👍", count: 2, reactors: [budi, us] },
  { emoji: "❤️", count: 1, reactors: [budi] },
];

describe("reactionToSend", () => {
  it("knows which emoji is ours", () => {
    expect(ownReaction(reactions)).toBe("👍");
    expect(ownReaction([reactions[1]])).toBeNull();
  });

  // Tapping our own reaction again is how WhatsApp takes it back.
  it("takes our reaction back when the same emoji is picked", () => {
    expect(reactionToSend(reactions, "👍")).toBe("");
  });

  it("replaces our reaction when another emoji is picked", () => {
    expect(reactionToSend(reactions, "❤️")).toBe("❤️");
    expect(reactionToSend([], "😂")).toBe("😂");
  });
});
