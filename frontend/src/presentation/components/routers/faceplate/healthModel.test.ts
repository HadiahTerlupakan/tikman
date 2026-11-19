import { describe, expect, it } from "vitest";
import type { RouterHealthItem } from "@/domain/entities";
import { mikrotikRouter } from "@/test/mikrotikRouterFixture";
import { healthView } from "./healthModel";

const NOW = new Date("2026-11-19T12:00:00Z");
const JUST_READ = "2026-11-19T11:59:00Z";

const item = (name: string, value: number, unit: number): RouterHealthItem => ({
  name,
  value,
  unit,
});

const ccr = [
  item("cpu-temperature", 43, 1),
  item("fan1-speed", 5654, 2),
  item("psu1-voltage", 121, 3),
  item("psu2-state", 1, 6),
  item("psu1-state", 0, 6),
  item("power-consumption", 452, 5),
];

const rb5009 = [
  item("cpu-temperature", 51, 1),
  item("board-temperature1", 38, 1),
];

const view = (health: RouterHealthItem[], over = {}) =>
  healthView(mikrotikRouter({ health, healthAt: JUST_READ, ...over }), NOW);

describe("healthView", () => {
  it("reads each PSU's state and names the failed one", () => {
    const v = view(ccr);

    expect(v.fresh).toBe(true);
    expect(v.psus).toEqual([
      { n: 1, ok: true },
      { n: 2, ok: false },
    ]);
    expect(v.failures).toEqual(["PSU2"]);
  });

  it("labels every reading and formats it by its unit", () => {
    const v = view(ccr);

    expect(v.readings.map((r) => [r.label, r.text, r.failed])).toEqual([
      ["Suhu CPU", "43 °C", false],
      ["Kipas 1", "5654 RPM", false],
      ["PSU1 tegangan", "12.1 V", false],
      ["psu2-state", "fail", true],
      ["psu1-state", "ok", false],
      ["Daya", "45.2 W", false],
    ]);
  });

  it("draws no PSU for a board that reports none", () => {
    const v = view(rb5009);

    expect(v.psus).toEqual([]);
    expect(v.failures).toEqual([]);
    expect(v.readings[1].label).toBe("Suhu board 1");
  });

  it("names a failed status item that is not a PSU by its own name", () => {
    expect(view([item("fan-state", 1, 6)]).failures).toEqual(["fan-state"]);
  });

  it("shows a status value that is neither 0 nor 1 raw and claims no PSU state from it", () => {
    const v = view([item("psu1-state", 0, 6), item("psu2-state", 2, 6)]);

    expect(v.psus).toEqual([
      { n: 1, ok: true },
      { n: 2, ok: null },
    ]);
    expect(v.failures).toEqual([]);
    expect(v.readings.map((r) => [r.text, r.failed])).toEqual([
      ["ok", false],
      ["2", false],
    ]);
  });

  it("keeps an unknown unit's value raw", () => {
    const v = view([item("new-sensor", 77, 9)]);

    expect(v.readings).toEqual([
      { name: "new-sensor", label: "new-sensor", text: "77", failed: false },
    ]);
  });

  it("treats a reading older than five minutes as stale", () => {
    expect(view(ccr, { healthAt: "2026-11-19T11:54:59Z" }).fresh).toBe(false);
  });

  it("treats an unreachable router's reading as stale", () => {
    expect(view(ccr, { status: "down" }).fresh).toBe(false);
  });

  it("has nothing fresh before the first reading", () => {
    const v = healthView(mikrotikRouter(), NOW);

    expect(v).toEqual({ fresh: false, psus: [], failures: [], readings: [] });
  });
});
