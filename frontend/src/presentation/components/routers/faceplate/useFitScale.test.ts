import { describe, expect, it } from "vitest";
import { MIN_STAGE_SCALE, stageScale } from "./useFitScale";

describe("stageScale", () => {
  it("shrinks a stage wider than its card to fit", () => {
    expect(stageScale(0.85)).toBe(0.85);
  });

  it("never enlarges a stage that already fits", () => {
    expect(stageScale(1.4)).toBe(1);
  });

  it("stops at the readable floor so a narrow screen scrolls instead", () => {
    expect(stageScale(0.3)).toBe(MIN_STAGE_SCALE);
  });

  it("keeps full size before the card has been measured", () => {
    expect(stageScale(0)).toBe(1);
    expect(stageScale(Number.NaN)).toBe(1);
  });
});
