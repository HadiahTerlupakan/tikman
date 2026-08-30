import { renderHook, act } from "@testing-library/react";
import { describe, expect, it, vi, beforeEach, afterEach } from "vitest";
import { useDebouncedValue } from "./useDebouncedValue";

describe("useDebouncedValue", () => {
  beforeEach(() => vi.useFakeTimers());
  afterEach(() => vi.useRealTimers());

  it("returns the first value without waiting", () => {
    // The list must render on load rather than after a delay that exists for
    // typing.
    const { result } = renderHook(() => useDebouncedValue("ZTEG", 300));

    expect(result.current).toBe("ZTEG");
  });

  it("holds a change back until the value settles", () => {
    const { result, rerender } = renderHook(
      ({ value }) => useDebouncedValue(value, 300),
      { initialProps: { value: "Z" } },
    );

    rerender({ value: "ZT" });
    rerender({ value: "ZTE" });
    expect(result.current).toBe("Z");

    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(result.current).toBe("ZTE");
  });

  it("emits only the last value of a burst", () => {
    // Twelve keystrokes of a serial should cost one query, not twelve.
    const { result, rerender } = renderHook(
      ({ value }) => useDebouncedValue(value, 300),
      { initialProps: { value: "" } },
    );

    for (const value of ["Z", "ZT", "ZTE", "ZTEG"]) {
      rerender({ value });
      act(() => {
        vi.advanceTimersByTime(100);
      });
    }

    expect(result.current).toBe("");

    act(() => {
      vi.advanceTimersByTime(300);
    });
    expect(result.current).toBe("ZTEG");
  });
});
