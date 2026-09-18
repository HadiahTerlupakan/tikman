import { useMemo, useState } from "react";
import type { MappingEdge, Waypoint } from "@/domain/entities";
import { metersAlong } from "./cableMath";

/** Tracing one cable: where it starts, the corners so far, and how long it is. */
export function useCableDraw() {
  const [from, setFrom] = useState<string>();
  const [points, setPoints] = useState<Waypoint[]>([]);
  const [redrawing, setRedrawing] = useState<MappingEdge>();

  const meters = useMemo(() => metersAlong(points), [points]);

  return {
    from,
    points,
    meters,
    /** The cable being retraced, when this trace is a redraw rather than a
     * fresh one — carried here so finish() knows which edge to save over. */
    redrawing,
    start: (nodeId: string) => {
      setFrom(nodeId);
      setPoints([]);
      setRedrawing(undefined);
    },
    // A redraw's endpoints are fixed by the cable already on record, so
    // tracing starts at its source instead of waiting for a node tap. No
    // corners carry over: retracing means tapping the whole path again, the
    // same gesture as drawing a fresh cable, not editing the old ones in place.
    startRedraw: (edge: MappingEdge) => {
      setFrom(edge.source);
      setPoints([]);
      setRedrawing(edge);
    },
    addPoint: (point: Waypoint) => setPoints((p) => [...p, point]),
    undoPoint: () => setPoints((p) => p.slice(0, -1)),
    finish: () => {
      const traced = points;
      setFrom(undefined);
      setPoints([]);
      setRedrawing(undefined);
      return traced;
    },
    cancel: () => {
      setFrom(undefined);
      setPoints([]);
      setRedrawing(undefined);
    },
  };
}
