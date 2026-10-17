import { describe, expect, it } from "vitest";
import { senderColor } from "./senderColor";

describe("senderColor", () => {
  // Derived from the jid alone, so a member keeps their colour whatever
  // order the thread or the member list happens to load in.
  it("gives a member the same colour whoever else is around", () => {
    const jids = ["628111@s.whatsapp.net", "628222@s.whatsapp.net"];
    const forward = jids.map(senderColor);
    const backward = [...jids].reverse().map(senderColor).reverse();
    expect(forward).toEqual(backward);
  });

  it("spreads different members over different colours", () => {
    const colours = new Set(
      Array.from({ length: 30 }, (_, i) =>
        senderColor(`62812${i}@s.whatsapp.net`),
      ),
    );
    expect(colours.size).toBeGreaterThan(5);
  });
});
