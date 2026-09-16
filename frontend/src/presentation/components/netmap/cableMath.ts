import type { Waypoint } from "@/domain/entities";

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
