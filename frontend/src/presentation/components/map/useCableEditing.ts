import { useState } from "react";
import { useSetCableRoute } from "@/application/hooks";
import type { RoutePoint } from "@/domain/entities";
import { anchoredRoute, type CableSegment } from "./cableSegments";

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
      return {
        ...selected,
        path: anchoredRoute(selected, drawn),
        traced: true,
      };
    },
  };
}
