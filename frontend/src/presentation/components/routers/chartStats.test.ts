import { describe, expect, it } from "vitest";
import { chartStats } from "./chartStats";
import type { ChartRow } from "./routerTraffic";

describe("chartStats", () => {
  it("gives each series its last, mean and peak value", () => {
    const rows = [
      { time: "2026-10-12T00:00:00Z", A: 10, B: 1 },
      { time: "2026-10-12T00:01:00Z", A: 30, B: 3 },
      { time: "2026-10-12T00:02:00Z", A: 20, B: 2 },
    ];

    expect(chartStats(rows, ["A", "B"])).toEqual([
      { name: "A", current: 20, average: 20, maximum: 30 },
      { name: "B", current: 2, average: 2, maximum: 3 },
    ]);
  });

  it("skips rows where a series has no sample instead of counting zero", () => {
    const rows: ChartRow[] = [
      { time: "2026-10-12T00:00:00Z", A: 10 },
      { time: "2026-10-12T00:01:00Z" },
      { time: "2026-10-12T00:02:00Z", A: 30 },
    ];

    expect(chartStats(rows, ["A"])[0]).toEqual({
      name: "A",
      current: 30,
      average: 20,
      maximum: 30,
    });
  });

  it("has no current value for a series missing from the last row", () => {
    const rows: ChartRow[] = [
      { time: "2026-10-12T00:00:00Z", A: 10, B: 5 },
      { time: "2026-10-12T00:01:00Z", A: 20 },
    ];

    expect(chartStats(rows, ["B"])[0].current).toBeNull();
  });

  it("reports nothing for a series with no samples", () => {
    expect(chartStats([{ time: "t" }], ["A"])[0]).toEqual({
      name: "A",
      current: null,
      average: null,
      maximum: null,
    });
  });
});
