import { describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import type { Waypoint } from "@/domain/entities";
import { useCableDraw } from "./useCableDraw";

describe("useCableDraw", () => {
  // A misplaced corner on a twelve-point trace should cost one tap to fix, not
  // the whole cable.
  it("takes back one corner, not the whole path", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.addPoint({ lat: -6.21, lng: 106.81 }));
    act(() => result.current.undoPoint());

    // Not just a length check: the point that survives has to be the first
    // one, not the second — a version that drops the wrong end still has
    // length 1 and would pass a weaker assertion.
    expect(result.current.points).toEqual([{ lat: -6.2, lng: 106.8 }]);
    expect(result.current.from).toBe("ODC-01");
  });

  it("measures the path as it is traced", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.addPoint({ lat: -6.21, lng: 106.8 }));

    expect(result.current.meters).toBeGreaterThan(1000);
  });

  it("hands back exactly the points traced, for finish to save", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.addPoint({ lat: -6.21, lng: 106.81 }));

    let traced: Waypoint[] = [];
    act(() => {
      traced = result.current.finish();
    });

    expect(traced).toEqual([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.21, lng: 106.81 },
    ]);
  });

  it("forgets everything when the drawing is abandoned", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.start("ODC-01"));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.cancel());

    expect(result.current.from).toBeUndefined();
    expect(result.current.points).toHaveLength(0);
  });
});
