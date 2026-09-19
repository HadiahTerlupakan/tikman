import type { MappingNode, Waypoint } from "@/domain/entities";

const EARTH_RADIUS_M = 6_371_000;

const toRad = (deg: number) => (deg * Math.PI) / 180;

function metersBetween(a: Waypoint, b: Waypoint): number {
  const dLat = toRad(b.lat - a.lat);
  const dLng = toRad(b.lng - a.lng);
  const h =
    Math.sin(dLat / 2) ** 2 +
    Math.cos(toRad(a.lat)) * Math.cos(toRad(b.lat)) * Math.sin(dLng / 2) ** 2;
  return 2 * EARTH_RADIUS_M * Math.asin(Math.sqrt(h));
}

/** The length of the path as drawn, corner by corner. */
export function metersAlong(points: Waypoint[]): number {
  return points
    .slice(1)
    .reduce((total, point, i) => total + metersBetween(points[i], point), 0);
}

export function formatMeters(m: number): string {
  if (m >= 1000) {
    return `${(m / 1000).toFixed(2).replace(".", ",")} km`;
  }
  return `${Math.round(m)} m`;
}

/**
 * The distance if this cable ran straight between its ends, ignoring
 * whatever corners it was actually traced through — what "how it was
 * traced" compares the drawn length against, to tell a route that was
 * walked on the ground from one that was only guessed as a straight line.
 */
export function straightLineMeters(
  source: MappingNode,
  target: MappingNode,
): number {
  return metersAlong([
    { lat: source.latitude, lng: source.longitude },
    { lat: target.latitude, lng: target.longitude },
  ]);
}

/**
 * The full drawn length of one cable: its named ends plus the corners traced
 * between them. An edge stores only the corners — `useCableDraw` never records
 * the node it started or finished on — so even a straight drop with no corners
 * at all needs both ends resolved before it is a path at all.
 *
 * This same walk has to serve a cable that is only a pending save
 * (NetworkMapPage, naming its ends directly) and one already on the map
 * (MapCanvas, naming them through a saved edge) — two separate copies of it is
 * exactly how the branch shipped a straight cable that measured 0 metres.
 *
 * Migration 53 keeps no foreign key from edge to node on purpose: a cable can
 * be drawn before its ends are named, and deleting a node here leaves its
 * cables behind rather than cascading them away. `undefined` here means only
 * "skip this one edge" — never a thrown error that would blank the rest of
 * the map or abort a save in progress.
 */
export function edgePath(
  edge: { source: string; target: string; waypoints: Waypoint[] | null },
  nodesById: Map<string, MappingNode>,
): Waypoint[] | undefined {
  const source = nodesById.get(edge.source);
  const target = nodesById.get(edge.target);
  if (!source || !target) {
    return undefined;
  }
  return [
    { lat: source.latitude, lng: source.longitude },
    ...(edge.waypoints ?? []),
    { lat: target.latitude, lng: target.longitude },
  ];
}

/**
 * The full path for a cable named by its two node ids rather than a saved
 * MappingEdge — what a page has on hand while a cable is only a pending save
 * or a redraw in progress, neither of which has an `edge_id` row to read
 * `waypoints` off of. Delegates to `edgePath` rather than walking the nodes
 * itself, so this can never drift back into the second implementation that
 * once shipped a straight cable measuring 0 metres.
 */
export function cablePath(
  nodes: MappingNode[],
  source: string,
  target: string,
  waypoints: Waypoint[],
): Waypoint[] {
  const nodesById = new Map(nodes.map((node) => [node.nodeId, node]));
  return edgePath({ source, target, waypoints }, nodesById) ?? [];
}
