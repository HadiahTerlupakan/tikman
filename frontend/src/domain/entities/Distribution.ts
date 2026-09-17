/**
 * An optical distribution point: the box a subscriber's drop cable lands in.
 *
 * It hangs off a cabinet (`odcId`) or off a PON port directly (`oltId` with
 * `slot` and `portId`), never both. `usedPorts` against `portCount` is what the
 * map colours by, because "is there room here" is the question it exists to
 * answer.
 */
export interface Odp {
  id: string;
  /** The box's identity, for the same reason a cabinet's is. */
  code: string;
  portCount: number;
  usedPorts: number;
  latitude?: number;
  longitude?: number;
  address: string;
  notes: string;
  odcId?: string;
  oltId?: string;
  slot?: number;
  portId?: number;
  /** The traced path, absent when the map should draw the straight line. */
  route?: RoutePoint[];
  routeMeters: number;
}

/** One vertex of a cable's path. */
export interface RoutePoint {
  lat: number;
  lng: number;
}
