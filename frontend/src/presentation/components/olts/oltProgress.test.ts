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

  // Re-polling an OLT TikMan already holds in full resets discoveryRegistered
  // to zero. Preferring that counter reported 0% on an OLT whose 200 ONTs all
  // had fresh metrics, for most of every cycle.
  it("keeps reporting metrics while a complete OLT is re-polled", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 200,
        ontsWithMetrics: 200,
        percentage: 0,
        phase: "polling",
        discoveryTotal: 200,
        discoveryRegistered: 0,
      }),
    ).toEqual({
      percent: 100,
      label: "Polling metrics",
      count: "200/200 ONTs polled",
    });
  });

  // An OLT that has grown since the last poll is building inventory again, so
  // the discovery counter is the honest figure until the new ONTs land.
  it("returns to discovery progress when the OLT reports more ONTs than are held", () => {
    expect(
      getOltProgressDisplay({
        totalOnts: 200,
        ontsWithMetrics: 200,
        percentage: 100,
        phase: "polling",
        discoveryTotal: 210,
        discoveryRegistered: 40,
      }),
    ).toEqual({
      percent: 19,
      label: "Discovering ONTs",
      count: "40/210 ONTs found",
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
