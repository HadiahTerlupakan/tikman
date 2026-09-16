import { describe, expect, it } from "vitest";
import { presetPeriod } from "./performancePeriod";

// 03:00 WIB on 16 September is still the 15th in UTC. The report's days are WIB
// days, so "today" here has to be the 16th whatever zone the browser is in.
const earlyMorningWIB = new Date("2026-09-15T20:00:00Z");

describe("presetPeriod", () => {
  it("takes today as the WIB day", () => {
    expect(presetPeriod("hari-ini", earlyMorningWIB)).toEqual({
      from: "2026-09-16",
      to: "2026-09-16",
    });
  });

  it("counts seven days including today", () => {
    expect(presetPeriod("7-hari", earlyMorningWIB)).toEqual({
      from: "2026-09-10",
      to: "2026-09-16",
    });
  });

  it("runs this month from its first day to today", () => {
    expect(presetPeriod("bulan-ini", earlyMorningWIB)).toEqual({
      from: "2026-09-01",
      to: "2026-09-16",
    });
  });

  it("covers the whole of last month", () => {
    expect(presetPeriod("bulan-lalu", earlyMorningWIB)).toEqual({
      from: "2026-08-01",
      to: "2026-08-31",
    });
  });

  it("reaches into last year in January", () => {
    expect(
      presetPeriod("bulan-lalu", new Date("2026-01-10T01:00:00Z")),
    ).toEqual({
      from: "2025-12-01",
      to: "2025-12-31",
    });
  });
});
