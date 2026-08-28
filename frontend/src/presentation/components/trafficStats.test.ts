import { describe, expect, it } from "vitest";
import { formatRateTick, percentile95, rateScaleFor } from "./trafficStats";

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

describe("rateScaleFor", () => {
  it("picks the unit the axis maximum calls for", () => {
    expect(rateScaleFor(2400).unit).toBe("Gbps");
    expect(rateScaleFor(12).unit).toBe("Mbps");
    expect(rateScaleFor(0.003).unit).toBe("Kbps");
  });

  it("falls back to Kbps for an empty axis", () => {
    expect(rateScaleFor(0).unit).toBe("Kbps");
  });
});

describe("formatRateTick", () => {
  // Rounding sub-unit ticks to whole numbers put "1K" on the axis twice.
  it("keeps a decimal on small ticks so two do not collide", () => {
    const scale = rateScaleFor(0.003);

    expect(formatRateTick(0.003, scale)).toBe("3.0");
    expect(formatRateTick(0.00225, scale)).toBe("2.3");
    expect(formatRateTick(0.0015, scale)).toBe("1.5");
    expect(formatRateTick(0.00075, scale)).toBe("0.8");
  });

  it("drops the decimal once the numbers are large enough to read", () => {
    const scale = rateScaleFor(120);

    expect(formatRateTick(120, scale)).toBe("120");
    expect(formatRateTick(30, scale)).toBe("30");
  });

  it("labels zero plainly", () => {
    expect(formatRateTick(0, rateScaleFor(12))).toBe("0");
  });
});
