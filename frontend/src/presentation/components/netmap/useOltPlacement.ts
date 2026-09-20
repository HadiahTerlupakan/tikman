import { useState } from "react";
import { message } from "antd";
import { useOlts, useUpdateOlt } from "@/application/hooks";
import type { Waypoint } from "@/domain/entities";
import {
  INVALID_COORDINATES_MESSAGE,
  isInvalidCoordinates,
} from "./mappingErrors";
import type { Placing } from "./MapToolbar";
import type { OltPlacementTarget } from "./NetworkMapModals";

/**
 * Placing a real OLT on the map: arming, capturing the tapped position, and
 * writing it onto the chosen OLT. The backend mirrors that write into a
 * mapping node (syncOLTMapNode) — refetchNodes is what makes that mirror
 * show up here, since useUpdateOlt only knows to invalidate its own OLT
 * queries and has no reason to know about mapping's.
 *
 * setPlacing is the page's own shared "what is armed" state (also cable and
 * every NodeType) — this hook only decides when to set or clear it at
 * "olt", never owning it outright, the same way it never owns `nodes`.
 */
export function useOltPlacement(
  setPlacing: (placing: Placing | undefined) => void,
  refetchNodes: () => Promise<unknown>,
) {
  const { data: olts = [] } = useOlts();
  const updateOlt = useUpdateOlt();
  const [target, setTarget] = useState<OltPlacementTarget>();

  // An OLT already on the map (both coordinates set) has nothing left to
  // place there — the same test the backend's own sync uses to decide
  // whether its mirror node exists at all.
  const unplaced = olts.filter(
    (olt) => olt.latitude === undefined || olt.longitude === undefined,
  );

  const arm = () => {
    // An empty Select would ask the operator to choose from nothing; saying
    // so up front is plainer than opening a picker that can only fail.
    if (unplaced.length === 0) {
      message.info("Semua OLT sudah punya posisi di peta");
      return;
    }
    setPlacing("olt");
  };

  const submit = async (oltId: string) => {
    if (!target) {
      return;
    }
    try {
      await updateOlt.mutateAsync({
        id: oltId,
        data: { latitude: target.position.lat, longitude: target.position.lng },
      });
      await refetchNodes();
      message.success("OLT ditempatkan di peta");
      setTarget(undefined);
      setPlacing(undefined);
    } catch (error) {
      message.error(
        isInvalidCoordinates(error)
          ? INVALID_COORDINATES_MESSAGE
          : "Gagal menempatkan OLT di peta",
      );
    }
  };

  return {
    olts,
    unplaced,
    target,
    arm,
    drop: (point: Waypoint) => setTarget({ position: point }),
    cancel: () => setTarget(undefined),
    submit,
  };
}
