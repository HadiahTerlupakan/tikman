import { describe, expect, it } from "vitest";
import type { WaGroupPoll } from "@/domain/entities";
import { nextVote, pollTally } from "./groupPoll";

const budi = { jid: "628111@s.whatsapp.net", name: "Budi", isSelf: false };
const us = { jid: "628999@s.whatsapp.net", name: "Nomor ini", isSelf: true };

function poll(over: Partial<WaGroupPoll> = {}): WaGroupPoll {
  return {
    question: "Kapan cek ODP?",
    selectableCount: 0,
    options: [
      { name: "Pagi", voters: [budi, us] },
      { name: "Siang", voters: [budi] },
      { name: "Sore", voters: [] },
    ],
    ...over,
  };
}

describe("pollTally", () => {
  // Two people voted; Budi backed two options. Measured against votes cast
  // Pagi would read 67%, but every voter chose it.
  it("measures each option against the people who voted", () => {
    const tally = pollTally(poll());
    expect(tally.voters).toBe(2);
    expect(tally.options.map((o) => [o.name, o.count, o.share])).toEqual([
      ["Pagi", 2, 1],
      ["Siang", 1, 0.5],
      ["Sore", 0, 0],
    ]);
    expect(tally.options[0].voterNames).toEqual(["Budi", "Anda"]);
    expect(tally.options[0].mine).toBe(true);
  });

  it("reads an unvoted poll as empty, not as a division by zero", () => {
    const tally = pollTally(poll({ options: [{ name: "Ya", voters: [] }] }));
    expect(tally.options[0].share).toBe(0);
  });
});

describe("nextVote", () => {
  it("adds to and withdraws from a multiple-choice selection", () => {
    expect(nextVote(poll(), "Siang")).toEqual(["Pagi", "Siang"]);
    expect(nextVote(poll(), "Pagi")).toEqual([]);
  });

  it("moves a single-choice vote rather than adding to it", () => {
    expect(nextVote(poll({ selectableCount: 1 }), "Sore")).toEqual(["Sore"]);
  });

  it("refuses a pick past the poll's limit", () => {
    const capped = poll({
      selectableCount: 2,
      options: [
        { name: "A", voters: [us] },
        { name: "B", voters: [us] },
        { name: "C", voters: [] },
      ],
    });
    expect(nextVote(capped, "C")).toBeNull();
    expect(nextVote(capped, "A")).toEqual(["B"]);
  });
});
