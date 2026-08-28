import { describe, expect, it } from "vitest";
import { formatBytes, formatRate } from "./trafficFormat";

describe("formatBytes", () => {
  // Lifetime totals: one subscriber on this OLT has moved 405 GB.
  it("scales a lifetime counter to a readable unit", () => {
    expect(formatBytes(405541528128)).toBe("377.69 GB");
    expect(formatBytes(45137724736)).toBe("42.04 GB");
  });

  it("leaves small values in bytes", () => {
    expect(formatBytes(512)).toBe("512 B");
    expect(formatBytes(0)).toBe("0 B");
  });

  // A counter the OLT did not report is not a counter reading zero.
  it("distinguishes a missing reading from none", () => {
    expect(formatBytes(undefined)).toBe("—");
    expect(formatBytes(null)).toBe("—");
  });
});

describe("formatRate", () => {
  it("uses Mbps at or above one", () => {
    expect(formatRate(1.138)).toBe("1.14 Mbps");
  });

  it("drops to Kbps for an idle link", () => {
    expect(formatRate(0.052)).toBe("52.00 Kbps");
    expect(formatRate(0.000004)).toBe("0.0040 Kbps");
  });

  it("shows a dash when no rate was returned", () => {
    expect(formatRate(null)).toBe("—");
  });
});
