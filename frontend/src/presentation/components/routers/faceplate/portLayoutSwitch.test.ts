import { describe, expect, it } from "vitest";
import { HUAWEI_CE_KINDS, layoutFaceplate } from "./portLayout";

const range = (prefix: string, from: number, to: number) =>
  Array.from({ length: to - from + 1 }, (_, i) => `${prefix}${from + i}`);

describe("layoutFaceplate for Huawei CE", () => {
  it("places the CE6850 ports like its front panel", () => {
    const names = [
      "MEth0/0/0",
      ...range("10GE1/0/", 1, 48),
      ...range("40GE1/0/", 1, 4),
    ];
    const layout = layoutFaceplate("CE6850-48S4Q-EI", names, HUAWEI_CE_KINDS);
    const by = Object.fromEntries(layout.jacks.map((j) => [j.name, j]));
    const tenGe = layout.jacks.filter((j) => j.name.startsWith("10GE"));

    expect(layout.templated).toBe(true);
    expect(layout.jacks).toHaveLength(53);
    expect(by["MEth0/0/0"]).toMatchObject({ kind: "rj", mgmt: true });
    expect(by["10GE1/0/1"]).toMatchObject({ kind: "sfp", row: "top" });
    expect(by["10GE1/0/1"].x).toBe(Math.min(...tenGe.map((j) => j.x)));
    expect(by["10GE1/0/2"]).toMatchObject({ row: "bottom" });
    expect(by["10GE1/0/2"].x).toBe(by["10GE1/0/1"].x);
    expect(by["MEth0/0/0"].x).toBeLessThan(by["10GE1/0/1"].x);
    expect(layout.jacks[layout.jacks.length - 1]).toMatchObject({
      name: "40GE1/0/4",
      kind: "qsfp",
    });
    expect(by["40GE1/0/1"].x).toBeGreaterThan(by["10GE1/0/48"].x);
  });

  it("does not put 10GE1/0/10 in the slot of 10GE1/0/1", () => {
    const layout = layoutFaceplate(
      "CE6850-48S4Q-EI",
      ["10GE1/0/10", "10GE1/0/1"],
      HUAWEI_CE_KINDS,
    );
    const first = layout.jacks.find((j) => j.slot === "10GE1/0/1");

    expect(first?.name).toBe("10GE1/0/1");
  });

  it("groups another CE model by Huawei names", () => {
    const layout = layoutFaceplate(
      "CE5850-48T4S2Q-EI",
      ["MEth0/0/0", "GE1/0/1", "GE1/0/2", "10GE1/0/1", "40GE1/0/1"],
      HUAWEI_CE_KINDS,
    );

    expect(layout.templated).toBe(false);
    expect(layout.jacks.map((j) => [j.name, j.kind])).toEqual([
      ["MEth0/0/0", "rj"],
      ["GE1/0/1", "rj"],
      ["GE1/0/2", "rj"],
      ["10GE1/0/1", "sfp"],
      ["40GE1/0/1", "qsfp"],
    ]);
    expect(layout.jacks[0].mgmt).toBe(true);
    expect(layout.jacks[1].mgmt).toBe(false);
  });
});
