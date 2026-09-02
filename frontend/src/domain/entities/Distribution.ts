/** An optical distribution cabinet: the first splitting stage after a PON port. */
export interface Odc {
  id: string;
  siteId: string;
  /** The cabinet's identity. There is no separate name. */
  code: string;
  latitude?: number;
  longitude?: number;
  address: string;
  notes: string;
  feedCount: number;
  odpCount: number;
}

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
}

/** One PON port feeding a cabinet, as the form states it. */
export interface CreateOdcFeedDto {
  oltId: string;
  slot: number;
  portId: number;
  splitterOutputs: number;
}

export interface CreateOdcDto {
  siteId: string;
  code: string;
  /** Saved with the cabinet, so a refused feed keeps neither. */
  feeds?: CreateOdcFeedDto[];
  latitude?: number;
  longitude?: number;
  address?: string;
  notes?: string;
}

export interface CreateOdpDto {
  code: string;
  portCount: number;
  latitude?: number;
  longitude?: number;
  address?: string;
  notes?: string;
  odcId?: string;
  oltId?: string;
  slot?: number;
  portId?: number;
}
