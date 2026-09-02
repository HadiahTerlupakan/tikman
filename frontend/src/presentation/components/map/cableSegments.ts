import type { Odc, OdcFeed, Odp, RoutePoint } from "@/domain/entities";

/** Somewhere a cable ends: an OLT, a cabinet, or a distribution box. */
interface Located {
  id: string;
  latitude?: number;
  longitude?: number;
}

/** One cable drawn on the map, with where it runs and how far. */
export interface CableSegment {
  id: string;
  kind: "feeder" | "distribution";
  path: RoutePoint[];
  meters: number;
  /** False when this is the straight line, not a path anyone traced. */
  traced: boolean;
}

const EARTH_RADIUS_METERS = 6371008.8;

/**
 * The distance between two points on the globe.
 *
 * The same haversine the backend uses for stored paths. It exists twice because
 * each side measures a different thing: the server measures the path someone
 * traced, this measures the straight line nobody has traced yet.
 */
export function metersBetween(from: RoutePoint, to: RoutePoint): number {
  const toRad = (degrees: number) => (degrees * Math.PI) / 180;
  const deltaLat = toRad(to.lat - from.lat);
  const deltaLng = toRad(to.lng - from.lng);
  const a =
    Math.sin(deltaLat / 2) ** 2 +
    Math.cos(toRad(from.lat)) *
      Math.cos(toRad(to.lat)) *
      Math.sin(deltaLng / 2) ** 2;
  return 2 * EARTH_RADIUS_METERS * Math.atan2(Math.sqrt(a), Math.sqrt(1 - a));
}

function pointOf(item: Located | undefined): RoutePoint | undefined {
  if (!item || item.latitude === undefined || item.longitude === undefined) {
    return undefined;
  }
  return { lat: item.latitude, lng: item.longitude };
}

function segment(
  id: string,
  kind: CableSegment["kind"],
  from: RoutePoint | undefined,
  to: RoutePoint | undefined,
  route: RoutePoint[] | undefined,
  routeMeters: number,
): CableSegment | undefined {
  if (route && route.length >= 2) {
    return { id, kind, path: route, meters: routeMeters, traced: true };
  }
  if (!from || !to) {
    return undefined;
  }
  return {
    id,
    kind,
    path: [from, to],
    meters: metersBetween(from, to),
    traced: false,
  };
}

/**
 * Every cable the map can draw.
 *
 * Read from the topology already recorded rather than from a list of cables: a
 * feeder is a cabinet's feed, a distribution cable is a box's parent link. A
 * cable whose ends are not both placed is left out — a line drawn from a
 * missing coordinate lands on the equator and claims a cable runs there.
 */
export function cableSegments(
  olts: Located[] | undefined,
  odcs: Odc[] | undefined,
  odps: Odp[] | undefined,
  feeds: OdcFeed[] | undefined,
): CableSegment[] {
  const oltById = new Map((olts ?? []).map((olt) => [olt.id, olt]));
  const odcById = new Map((odcs ?? []).map((odc) => [odc.id, odc]));
  const segments: CableSegment[] = [];

  for (const feed of feeds ?? []) {
    const drawn = segment(
      `feed-${feed.id}`,
      "feeder",
      pointOf(oltById.get(feed.oltId)),
      pointOf(odcById.get(feed.odcId)),
      feed.route,
      feed.routeMeters,
    );
    if (drawn) segments.push(drawn);
  }

  for (const odp of odps ?? []) {
    const parent = odp.odcId
      ? pointOf(odcById.get(odp.odcId))
      : pointOf(odp.oltId ? oltById.get(odp.oltId) : undefined);
    const drawn = segment(
      `odp-${odp.id}`,
      "distribution",
      parent,
      pointOf(odp),
      odp.route,
      odp.routeMeters,
    );
    if (drawn) segments.push(drawn);
  }

  return segments;
}

/**
 * The path to store for a cable, anchored to its real ends.
 *
 * Someone traces the poles in between; the first and last legs are added here
 * so the cable always starts and finishes where the plant actually is. Without
 * that, a route saved from a few clicks near the middle would draw a cable
 * floating short of both ends.
 */
export function anchoredRoute(
  segment: CableSegment,
  drawn: RoutePoint[],
): RoutePoint[] {
  const start = segment.path[0];
  const end = segment.path[segment.path.length - 1];
  return [start, ...drawn, end];
}
