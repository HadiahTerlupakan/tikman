import { useState } from "react";
import type { MappingEdge, MappingNode } from "@/domain/entities";

/**
 * Which marker or cable's popup is open on the map, if any. The two are
 * mutually exclusive — only one InfoWindow can sensibly anchor to the map at
 * a time — so selecting one kind always closes the other rather than the two
 * stacking.
 */
export function useMapSelection() {
  const [node, setNode] = useState<MappingNode>();
  const [edge, setEdge] = useState<MappingEdge>();

  return {
    node,
    edge,
    selectNode: (target: MappingNode) => {
      setEdge(undefined);
      setNode(target);
    },
    selectEdge: (target: MappingEdge) => {
      setNode(undefined);
      setEdge(target);
    },
    clear: () => {
      setNode(undefined);
      setEdge(undefined);
    },
  };
}
