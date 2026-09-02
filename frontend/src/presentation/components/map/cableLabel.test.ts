import { describe, expect, it } from "vitest";
import { cableKindLabel, cableLengthLabel } from "./cableLabel";
import type { CableSegment } from "./cableSegments";

const segment = (over: Partial<CableSegment> = {}): CableSegment => ({
  id: "odp-1",
  kind: "distribution",
  path: [
    { lat: 0, lng: 0 },
    { lat: 0, lng: 1 },
  ],
  meters: 420,
  traced: true,
  ...over,
});

describe("cableLengthLabel", () => {
  it("says metres for a short run", () => {
    expect(cableLengthLabel(segment({ meters: 420.4 }))).toBe("420 m");
  });

  it("switches to kilometres once the number stops being readable", () => {
    expect(cableLengthLabel(segment({ meters: 2540 }))).toBe("2.54 km");
  });

  it("says so when the number is only the gap between two points", () => {
    // A cable following poles is always longer than the straight line. Without
    // saying so, the smaller number gets read as cable length and ordered on.
    expect(cableLengthLabel(segment({ meters: 420, traced: false }))).toBe(
      "420 m (garis lurus)",
    );
  });
});

describe("cableKindLabel", () => {
  it("names each kind the way an operator does", () => {
    expect(cableKindLabel(segment({ kind: "feeder" }))).toBe("Feeder ke ODC");
    expect(cableKindLabel(segment({ kind: "distribution" }))).toBe(
      "Distribusi ke ODP",
    );
  });
});
