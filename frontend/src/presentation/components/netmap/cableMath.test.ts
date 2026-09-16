import { describe, expect, it } from "vitest";
import { formatMeters, metersAlong } from "./cableMath";

describe("metersAlong", () => {
  // The length shown has to be the cable that was pulled, not the straight line
  // between its ends — that is the whole reason waypoints are stored.
  it("measures the path drawn, not the line between its ends", () => {
    const straight = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    const detoured = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.2, lng: 106.81 },
      { lat: -6.21, lng: 106.81 },
      { lat: -6.21, lng: 106.8 },
    ]);

    expect(detoured).toBeGreaterThan(straight);
  });

  it("is zero for a path that goes nowhere", () => {
    expect(metersAlong([])).toBe(0);
    expect(metersAlong([{ lat: -6.2, lng: 106.8 }])).toBe(0);
  });

  it("measures about a kilometre for a hundredth of a degree of latitude", () => {
    const m = metersAlong([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.8 },
    ]);
    expect(m).toBeGreaterThan(1050);
    expect(m).toBeLessThan(1160);
  });
});

describe("formatMeters", () => {
  it("reads in metres under a kilometre and in kilometres above it", () => {
    expect(formatMeters(240)).toBe("240 m");
    expect(formatMeters(1250)).toBe("1,25 km");
  });
});
