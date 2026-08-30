import { describe, expect, it } from "vitest";
import { coordinateError, parseCoordinate } from "./siteCoordinates";

describe("parseCoordinate", () => {
  it("reads a decimal degree with either sign", () => {
    expect(parseCoordinate("-6.4025")).toBeCloseTo(-6.4025);
    expect(parseCoordinate("106.7942")).toBeCloseTo(106.7942);
  });

  it("tolerates the spaces a paste brings with it", () => {
    expect(parseCoordinate("  -6.4025 ")).toBeCloseTo(-6.4025);
  });

  it("is null for anything that is not a number", () => {
    expect(parseCoordinate("")).toBeNull();
    expect(parseCoordinate("north")).toBeNull();
    expect(parseCoordinate("6,4025")).toBeNull();
  });
});

describe("coordinateError", () => {
  it("accepts both fields empty, because a site need not be mapped", () => {
    expect(coordinateError("", "")).toBeNull();
  });

  it("accepts a valid pair", () => {
    expect(coordinateError("-6.4025", "106.7942")).toBeNull();
  });

  it("refuses one without the other", () => {
    // Half a coordinate would put a pin on the prime meridian and look
    // deliberate.
    expect(coordinateError("-6.4025", "")).toMatch(/together/i);
    expect(coordinateError("", "106.7942")).toMatch(/together/i);
  });

  it("refuses a point that cannot exist", () => {
    expect(coordinateError("91", "0")).toMatch(/-90/);
    expect(coordinateError("0", "181")).toMatch(/-180/);
  });

  it("refuses text that is not a number at all", () => {
    expect(coordinateError("north", "106.7942")).toMatch(/number/i);
  });
});
