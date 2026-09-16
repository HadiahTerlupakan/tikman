import { describe, expect, it } from "vitest";
import {
  END_REASON_LABELS,
  formatMinutes,
  formatPercent,
  formatPeriod,
} from "./performanceFormat";

describe("formatMinutes", () => {
  it("says nothing was measured rather than showing zero", () => {
    expect(formatMinutes(null)).toBe("—");
    expect(formatMinutes(undefined)).toBe("—");
  });

  it("keeps one decimal under an hour", () => {
    expect(formatMinutes(4.2)).toBe("4,2 mnt");
    expect(formatMinutes(0)).toBe("0 mnt");
  });

  it("switches to hours before it would ever print 60 minutes", () => {
    expect(formatMinutes(59.96)).toBe("1 j");
    expect(formatMinutes(125)).toBe("2 j 5 mnt");
  });
});

describe("formatPercent", () => {
  it("rounds to a whole percent", () => {
    expect(formatPercent(86.4)).toBe("86%");
  });

  it("says nothing was measured rather than showing zero", () => {
    expect(formatPercent(null)).toBe("—");
    expect(formatPercent(undefined)).toBe("—");
    expect(formatPercent(0)).toBe("0%");
  });
});

describe("END_REASON_LABELS", () => {
  it("names every way a wait can end", () => {
    expect(END_REASON_LABELS.replied).toBe("Dibalas");
    expect(END_REASON_LABELS.phone).toBe("Dibalas dari HP");
    expect(END_REASON_LABELS.closed).toBe("Ditutup tanpa balasan");
    expect(END_REASON_LABELS.abandoned).toBe("Ditinggal");
  });
});

describe("formatPeriod", () => {
  it("names a single day once, not as a same-day range", () => {
    expect(formatPeriod({ from: "2026-09-16", to: "2026-09-16" })).toBe(
      "16 Sep 2026",
    );
  });

  it("names a range with both ends", () => {
    expect(formatPeriod({ from: "2026-09-01", to: "2026-09-16" })).toBe(
      "1 Sep 2026 – 16 Sep 2026",
    );
  });
});
