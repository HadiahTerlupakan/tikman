import type { MappingEdge, MappingNode } from "@/domain/entities";
import { ApiError } from "@/infrastructure/http";

// A capacity rule on an odp_to_odp/odc_to_odc cascade is also a 409, and must
// reach the operator as itself; only the id collision this cable's own
// `source--target` naming produces should read as "already exists". Branching
// on the code the backend now sends (not the shared status) is what tells
// them apart.
export function isEdgeExists(error: unknown): boolean {
  return error instanceof ApiError && error.code === "EDGE_EXISTS";
}

export function isNodeExists(error: unknown): boolean {
  return error instanceof ApiError && error.code === "NODE_EXISTS";
}

export function isNodeInUse(error: unknown): boolean {
  return error instanceof ApiError && error.code === "NODE_IN_USE";
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Gagal menyimpan kabel";
}

/**
 * The message to refuse a redraw with, if either of the edge's ends is no
 * longer on the map — or `undefined` if both are still there. Node deletion
 * never cascades to edges (migration 53's own design), so a cable's stored
 * source or target can be gone by the time it is retraced.
 *
 * `cablePath`'s `?? []` fallback exists so `MapCanvas` can skip *drawing* a
 * dangling edge; reusing that fallback for arithmetic would silently
 * recompute `metersAlong([]) === 0` — the exact historical bug (every
 * straight cable recording 0m) reopened through the only door a redraw can
 * open onto it. Checking here, before that arithmetic ever runs, is what
 * keeps this a refusal instead of a silently corrupted save.
 */
export function missingEndpointMessage(
  nodes: MappingNode[],
  edge: MappingEdge,
): string | undefined {
  const exists = (nodeId: string) => nodes.some((n) => n.nodeId === nodeId);
  if (!exists(edge.source)) {
    return `Node sumber "${edge.source}" sudah dihapus dari peta, kabel tidak bisa disimpan`;
  }
  if (!exists(edge.target)) {
    return `Node tujuan "${edge.target}" sudah dihapus dari peta, kabel tidak bisa disimpan`;
  }
  return undefined;
}
