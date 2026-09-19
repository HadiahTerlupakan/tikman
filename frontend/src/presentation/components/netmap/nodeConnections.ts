import type {
  FiberType,
  MappingEdge,
  MappingNode,
  NodeType,
} from "@/domain/entities";

export interface IncomingConnection {
  fiberType: FiberType;
  /** The node this cable comes from, or undefined once it has been deleted —
   * node deletion never cascades to edges (see EdgePopup's own comment on
   * the same fact), so this is a normal, reachable state. */
  node?: MappingNode;
}

export interface OutgoingCount {
  type: NodeType;
  count: number;
}

export interface NodeConnections {
  incoming: IncomingConnection[];
  outgoing: OutgoingCount[];
}

// Fixed order so the "Ke:" line always reads the same way regardless of the
// order cables happen to have been drawn or returned by the API.
const TYPE_ORDER: NodeType[] = ["server", "odc", "odp", "ont"];

/**
 * What feeds a node and what hangs off it, counted by kind. Pure, over the
 * `edges`/`nodesById` the map page already has loaded — no request of its
 * own. A dangling edge (the node at its other end deleted) must not throw
 * and must not be guessed into the wrong kind: an incoming one is still
 * listed with `node: undefined` since it is still one named cable, while an
 * outgoing one is left out of the by-kind counts because its kind cannot be
 * known at all.
 */
export function nodeConnections(
  node: MappingNode,
  edges: MappingEdge[],
  nodesById: Map<string, MappingNode>,
): NodeConnections {
  const incoming = edges
    .filter((edge) => edge.target === node.nodeId)
    .map((edge) => ({
      fiberType: edge.fiberType,
      node: nodesById.get(edge.source),
    }));

  const counts = new Map<NodeType, number>();
  edges
    .filter((edge) => edge.source === node.nodeId)
    .forEach((edge) => {
      const target = nodesById.get(edge.target);
      if (target) {
        counts.set(target.type, (counts.get(target.type) ?? 0) + 1);
      }
    });
  const outgoing = Array.from(counts, ([type, count]) => ({
    type,
    count,
  })).sort((a, b) => TYPE_ORDER.indexOf(a.type) - TYPE_ORDER.indexOf(b.type));

  return { incoming, outgoing };
}
