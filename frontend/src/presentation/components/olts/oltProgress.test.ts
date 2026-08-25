import { describe, expect, it } from "vitest";
import { getOltProgressDisplay } from "./oltProgress";

describe("getOltProgressDisplay", () => {
  it("treats an idle OLT with no ONTs as discovery waiting", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 0,
        ontsWithMetrics: 0,
        percentage: 0,
      }),
    ).toEqual({
      percent: 0,
      label: "Discovering ONTs…",
      count: "Waiting for OLT",
    });
  });

  it("shows discovery as waiting before the OLT reports its ONT total", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 0,
        ontsWithMetrics: 0,
        percentage: 0,
        phase: "discovering",
        discoveryTotal: 0,
        discoveryRegistered: 0,
      }),
    ).toEqual({
      percent: 0,
      label: "Discovering ONTs…",
      count: "Waiting for OLT",
    });
  });

  it("uses registered ONTs for incremental discovery progress", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 25,
        ontsWithMetrics: 0,
        percentage: 13,
        phase: "polling",
        discoveryTotal: 197,
        discoveryRegistered: 25,
      }),
    ).toEqual({
      percent: 13,
      label: "Discovering ONTs",
      count: "25/197 ONTs found",
    });
  });

  it("uses fresh metrics after discovery completes", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 197,
        ontsWithMetrics: 192,
        percentage: 97.46,
        phase: "completed",
        discoveryTotal: 197,
        discoveryRegistered: 197,
      }),
    ).toEqual({
      percent: 97,
      label: "Polling metrics",
      count: "192/197 ONTs polled",
    });
  });
});
