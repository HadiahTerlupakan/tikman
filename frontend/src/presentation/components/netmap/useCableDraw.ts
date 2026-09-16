import { useMemo, useState } from "react";
import type { Waypoint } from "@/domain/entities";
import { metersAlong } from "./cableMath";

/** Tracing one cable: where it starts, the corners so far, and how long it is. */
export function useCableDraw() {
  const [from, setFrom] = useState<string>();
  const [points, setPoints] = useState<Waypoint[]>([]);

  const meters = useMemo(() => metersAlong(points), [points]);

  return {
    from,
    points,
    meters,
    start: (nodeId: string) => {
      setFrom(nodeId);
      setPoints([]);
    },
    addPoint: (point: Waypoint) => setPoints((p) => [...p, point]),
    undoPoint: () => setPoints((p) => p.slice(0, -1)),
    finish: () => {
      const traced = points;
      setFrom(undefined);
      setPoints([]);
      return traced;
    },
    cancel: () => {
      setFrom(undefined);
      setPoints([]);
    },
  };
}
