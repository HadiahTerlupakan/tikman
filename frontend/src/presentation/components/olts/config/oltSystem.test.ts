import { describe, expect, it } from "vitest";
import { formatUptime, groupPortsBySlot, summariseModel } from "./oltSystem";
import type { OltPort } from "@/domain/entities";

function port(overrides: Partial<OltPort> = {}): OltPort {
  return {
    ifIndex: 1,
    name: "gpon_1/3/1",
    kind: "pon",
    rack: 1,
    slot: 3,
    port: 1,
    adminUp: true,
    operUp: true,
    adminStatus: 1,
    operStatus: 1,
    ...overrides,
  };
}

describe("groupPortsBySlot", () => {
  it("keeps only the requested kind and groups by card", () => {
    const groups = groupPortsBySlot(
      [
        port({ slot: 4, port: 2 }),
        port({ slot: 3, port: 1 }),
        port({ slot: 10, port: 1, kind: "uplink", name: "xgei_1/10/1" }),
      ],
      "pon",
    );

    expect(groups.map((group) => group.slot)).toEqual([3, 4]);
  });

  it("orders ports within a card numerically", () => {
    const groups = groupPortsBySlot(
      [port({ port: 10 }), port({ port: 2 }), port({ port: 1 })],
      "pon",
    );

    expect(groups[0].ports.map((p) => p.port)).toEqual([1, 2, 10]);
  });

  // The management interface has no slot address, so it must not be drawn into
  // the PON or uplink grids.
  it("leaves out interfaces of another kind", () => {
    expect(
      groupPortsBySlot([port({ kind: "other", name: "Mng1" })], "uplink"),
    ).toEqual([]);
  });
});

describe("formatUptime", () => {
  it("reports days, hours and minutes", () => {
    expect(formatUptime(2473174)).toBe("28d 14h 59m");
  });

  it("omits days and hours below a day", () => {
    expect(formatUptime(90)).toBe("1m");
  });

  it("says so when the OLT has not reported an uptime", () => {
    expect(formatUptime(0)).toBe("unknown");
  });
});

describe("summariseModel", () => {
  it("reduces the sysDescr banner to model and version", () => {
    expect(
      summariseModel(
        "C300 Version V2.1.0 Software, Copyright (c) by ZTE Corporation Compiled",
      ),
    ).toBe("C300 V2.1.0");
  });

  it("falls back to the first clause when the banner is shaped differently", () => {
    expect(summariseModel("Some OLT, build 7")).toBe("Some OLT");
  });
});
