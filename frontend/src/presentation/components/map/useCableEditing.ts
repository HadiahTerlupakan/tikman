import { useState } from "react";
import { useSetCableRoute } from "@/application/hooks";
import type { RoutePoint } from "@/domain/entities";
import {
  anchoredRoute,
  metersBetween,
  type CableSegment,
} from "./cableSegments";

/**
 * Selecting a cable and tracing its path.
 *
 * The id a segment carries says which record the path belongs to — `feed-<id>`
 * for a feeder, `odp-<id>` for a distribution cable — because the cables are
 * derived from the plant rather than stored as their own rows.
 */
export function useCableEditing() {
  const [selected, setSelected] = useState<CableSegment>();
  const [drafting, setDrafting] = useState(false);
  const [drawn, setDrawn] = useState<RoutePoint[]>([]);
  const save = useSetCableRoute();

  const close = () => {
    setSelected(undefined);
    setDrafting(false);
    setDrawn([]);
  };

  const store = async (route: RoutePoint[]) => {
    if (!selected) {
      return;
    }
    await save.mutateAsync({
      kind: selected.kind,
      id: selected.id.replace(/^(feed|odp)-/, ""),
      route,
    });
    close();
  };

  return {
    selected,
    drafting,
    drawn,
    saving: save.isPending,
    select: (segment: CableSegment) => {
      setSelected(segment);
      setDrafting(false);
      setDrawn([]);
    },
    startDraw: () => {
      setDrafting(true);
      setDrawn([]);
    },
    addPoint: (point: RoutePoint) => setDrawn((points) => [...points, point]),
    // A misplaced click on a twelve-point trace should cost one click to fix,
    // not the whole route.
    undoPoint: () => setDrawn((points) => points.slice(0, -1)),
    close,
    /** Saves what was traced, anchored to the cable's real ends. */
    saveDraft: () => selected && store(anchoredRoute(selected, drawn)),
    /** An empty path hands the cable back to the straight line. */
    straighten: () => store([]),
    /** The path as it stands, drawn over the cable it will replace. */
    draftSegment: (): CableSegment | undefined => {
      if (!drafting || !selected) {
        return undefined;
      }
      const path = anchoredRoute(selected, drawn);
      // Measured as it is traced, so the length on screen is the path being
      // drawn rather than the straight line it is replacing.
      const meters = path
        .slice(1)
        .reduce((total, point, i) => total + metersBetween(path[i], point), 0);
      return { ...selected, path, meters, traced: true };
    },
  };
}
