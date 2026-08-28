import { describe, expect, it } from "vitest";
import { percentile95 } from "./trafficStats";

describe("percentile95", () => {
  // Nearest-rank over 100 samples puts the answer at the 95th.
  it("takes the value at the 95th rank", () => {
    const values = Array.from({ length: 100 }, (_, i) => i + 1);

    expect(percentile95(values)).toBe(95);
  });

  // The point of the measure: one brief spike must not become the figure the
  // whole period is judged by.
  it("discards a lone spike", () => {
    const values = [...Array.from({ length: 99 }, () => 10), 1000];

    expect(percentile95(values)).toBe(10);
  });

  it("does not reorder the caller's array", () => {
    const values = [5, 1, 3];
    percentile95(values);

    expect(values).toEqual([5, 1, 3]);
  });

  // A period with no samples has no percentile, which is not the same as zero.
  it("reports nothing for an empty period", () => {
    expect(percentile95([])).toBeUndefined();
  });
});
