import type {
  FiberType,
  MappingEdge,
  MappingNode,
  NodeType,
} from "@/domain/entities";

interface SlotRule {
  source: NodeType;
  target: NodeType;
  /** Set only for a cascade. The backend matches the fiber type exactly, so
   * a "_ratio" variant of the same cascade is a different FiberType and is
   * not this rule — nor any other, same as the backend. */
  fiber?: FiberType;
}

// One entry per case in slotKind() (backend/internal/services/mapping_edges.go),
// in the same order, so a change on one side is easy to check against the other.
const SLOT_RULES: SlotRule[] = [
  { source: "odc", target: "odp" },
  { source: "odc", target: "odc", fiber: "odc_to_odc" },
  { source: "odp", target: "ont" },
  { source: "odp", target: "odp", fiber: "odp_to_odp" },
];

export interface SlotUsage {
  targetType: NodeType;
  /** True for a cascade to another node of the source's own kind — counted
   * against a separate pool from ordinary drops, per checkSlots's kind.fiber
   * filter. */
  cascade: boolean;
  used: number;
}

export interface NodeSlotInfo {
  capacity: number;
  /** Mirrors checkSlots' own capacity <= 0 rule: nobody has counted this
   * box's ports, so it is never refused and always reads as unlimited. */
  unlimited: boolean;
  usages: SlotUsage[];
}

function matchesRule(
  edge: MappingEdge,
  sourceId: string,
  rule: SlotRule,
  nodesById: Map<string, MappingNode>,
): boolean {
  if (edge.source !== sourceId) {
    return false;
  }
  const target = nodesById.get(edge.target);
  if (!target || target.type !== rule.target) {
    return false;
  }
  return rule.fiber === undefined || edge.fiberType === rule.fiber;
}

/**
 * Mirrors slotKind() + the counting half of checkSlots() in
 * backend/internal/services/mapping_edges.go. There is no shared code
 * between Go and TypeScript, so keep the two in sync by hand: get this
 * wrong and the popup tells a technician there is room when the backend's
 * save is about to refuse it, or the reverse.
 *
 * A cascade to another node of the source's own kind (ODC→ODC, ODP→ODP) is
 * counted against a separate pool from ordinary drops, because the backend's
 * `kind.fiber` filter keeps them apart — an ODC with 8 ODPs and 2 ODC
 * cascades is not "10 of 8", it is two entries here. Server and ONT nodes
 * are never a `source` in any backend rule, so they get `undefined` back:
 * there is nothing to count, unlimited or otherwise.
 */
export function slotUsage(
  node: MappingNode,
  edges: MappingEdge[],
  nodesById: Map<string, MappingNode>,
): NodeSlotInfo | undefined {
  const rules = SLOT_RULES.filter((rule) => rule.source === node.type);
  if (rules.length === 0) {
    return undefined;
  }
  if (node.capacity <= 0) {
    return { capacity: node.capacity, unlimited: true, usages: [] };
  }
  const usages = rules.map((rule) => ({
    targetType: rule.target,
    cascade: rule.fiber !== undefined,
    used: edges.filter((edge) =>
      matchesRule(edge, node.nodeId, rule, nodesById),
    ).length,
  }));
  return { capacity: node.capacity, unlimited: false, usages };
}
