import { describe, expect, it } from "vitest";
import { act, renderHook } from "@testing-library/react";
import type { MappingEdge, Waypoint } from "@/domain/entities";
import { useCableDraw } from "./useCableDraw";

// A cable already on record, as it would come back from the API — geometry
// already traced, unlike a fresh cable still being drawn.
const existingEdge: MappingEdge = {
  edgeId: "ODC-01--ODP-01",
  source: "ODC-01",
  target: "ODP-01",
  fiberType: "distribution",
  distance: 120,
  waypoints: [{ lat: -6.205, lng: 106.805 }],
  notes: "Kabel lama",
};

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

  // Redrawing starts tracing at the cable's existing source rather than
  // waiting for a node tap, since the endpoints are fixed and only the route
  // between them is being redone.
  it("starts a redraw at the cable's existing source, carrying no corners over", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.startRedraw(existingEdge));

    // Not seeded from existingEdge.waypoints: retracing means tapping the
    // whole path again, the same gesture as drawing a fresh cable — not
    // editing the old corners in place.
    expect(result.current.from).toBe("ODC-01");
    expect(result.current.points).toEqual([]);
    expect(result.current.redrawing).toBe(existingEdge);
  });

  it("clears which cable is being redrawn once the retrace is finished", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.startRedraw(existingEdge));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => {
      result.current.finish();
    });

    expect(result.current.redrawing).toBeUndefined();
    expect(result.current.from).toBeUndefined();
    expect(result.current.points).toHaveLength(0);
  });

  it("clears which cable is being redrawn when the retrace is cancelled", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.startRedraw(existingEdge));
    act(() => result.current.addPoint({ lat: -6.2, lng: 106.8 }));
    act(() => result.current.cancel());

    expect(result.current.redrawing).toBeUndefined();
    expect(result.current.from).toBeUndefined();
    expect(result.current.points).toHaveLength(0);
  });

  // The state machine is shared between the two entry points; starting a
  // fresh cable must leave no stale redraw target behind for finish() to
  // save over by mistake.
  it("leaves redraw mode behind when a fresh trace is started instead", () => {
    const { result } = renderHook(() => useCableDraw());

    act(() => result.current.startRedraw(existingEdge));
    act(() => result.current.start("ODC-02"));

    expect(result.current.redrawing).toBeUndefined();
    expect(result.current.from).toBe("ODC-02");
  });
});
