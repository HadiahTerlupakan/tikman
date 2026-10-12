import { describe, expect, it } from "vitest";
import {
  liveRefetchInterval,
  resolveTrafficRange,
  trafficRangeKey,
} from "./trafficRange";

const now = new Date("2026-10-12T09:00:00.000Z");

describe("resolveTrafficRange", () => {
  it("counts a relative range back from the moment it is resolved", () => {
    expect(resolveTrafficRange({ kind: "relative", hours: 6 }, now)).toEqual({
      from: "2026-10-12T03:00:00.000Z",
      to: "2026-10-12T09:00:00.000Z",
    });
  });

  it("moves a relative range forward when resolved later", () => {
    const later = new Date("2026-10-12T09:01:00.000Z");
    expect(resolveTrafficRange({ kind: "relative", hours: 6 }, later).to).toBe(
      "2026-10-12T09:01:00.000Z",
    );
  });

  it("returns an absolute range as picked, whatever the time", () => {
    const range = {
      kind: "absolute" as const,
      from: "2026-10-01T00:00:00.000Z",
      to: "2026-10-02T00:00:00.000Z",
    };
    expect(resolveTrafficRange(range, now)).toEqual({
      from: range.from,
      to: range.to,
    });
  });
});

describe("trafficRangeKey", () => {
  it("keys a relative range on its length only, so the clock does not churn the cache", () => {
    expect(trafficRangeKey({ kind: "relative", hours: 24 })).toEqual([
      "last",
      24,
    ]);
  });

  it("keys an absolute range on its endpoints", () => {
    expect(trafficRangeKey({ kind: "absolute", from: "a", to: "b" })).toEqual([
      "between",
      "a",
      "b",
    ]);
  });
});

describe("liveRefetchInterval", () => {
  it("refreshes a relative range and leaves an absolute one alone", () => {
    expect(liveRefetchInterval({ kind: "relative", hours: 6 }, 60_000)).toBe(
      60_000,
    );
    expect(
      liveRefetchInterval({ kind: "absolute", from: "a", to: "b" }, 60_000),
    ).toBe(false);
  });
});
