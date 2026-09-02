import type { Odc, Odp } from "@/domain/entities";

/** Plant the map can actually draw, with both coordinates known to be present. */
export type Placed<T> = T & { latitude: number; longitude: number };

function isPlaced<T extends { latitude?: number; longitude?: number }>(
  item: T,
): item is Placed<T> {
  return (
    typeof item.latitude === "number" && typeof item.longitude === "number"
  );
}

/**
 * The cabinets or boxes carrying both coordinates.
 *
 * One coordinate is not a location: drawing it would put the box on the equator
 * or the meridian and claim that is where it is.
 */
export function mappedPlant<
  T extends { latitude?: number; longitude?: number },
>(items: T[] | undefined): Placed<T>[] {
  return (items ?? []).filter(isPlaced);
}

/** How full a box is, from 0 to 1. A box recorded with no ports has no room. */
export function odpFullness(odp: Odp): number {
  if (odp.portCount <= 0) {
    return 1;
  }
  return Math.min(1, odp.usedPorts / odp.portCount);
}

// Amber before red, because a box down to its last port still needs planning
// for and should not read the same as a half-empty one.
const NEARLY_FULL = 0.75;
const PIN_ROOM = "#3ecf8e";
const PIN_NEARLY_FULL = "#f59e0b";
const PIN_FULL = "#ef4444";

/**
 * The colour a distribution box is drawn in.
 *
 * "Is there room here" is the question the map exists to answer, and answering
 * it in the pin means answering it without opening anything.
 */
export function odpPinColor(odp: Odp): string {
  const fullness = odpFullness(odp);
  if (fullness >= 1) {
    return PIN_FULL;
  }
  return fullness >= NEARLY_FULL ? PIN_NEARLY_FULL : PIN_ROOM;
}

/** How a box's occupancy reads in words, for a label beside the pin. */
export function odpOccupancyLabel(odp: Odp): string {
  return `${odp.usedPorts}/${odp.portCount} port terpakai`;
}

/** How a cabinet's fan-out reads in words. */
export function odcSummaryLabel(odc: Odc): string {
  return `${odc.feedCount} feed · ${odc.odpCount} ODP`;
}
