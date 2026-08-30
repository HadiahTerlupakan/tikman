import type { Olt } from "@/domain/entities";

/** An OLT the map can actually draw, with both coordinates known to be present. */
export type MappedOlt = Olt & { latitude: number; longitude: number };

function isMapped(olt: Olt): olt is MappedOlt {
  return typeof olt.latitude === "number" && typeof olt.longitude === "number";
}

/** OLTs carrying both coordinates, which are the only ones that can be drawn. */
export function mappedOlts(olts: Olt[] | undefined): MappedOlt[] {
  return (olts ?? []).filter(isMapped);
}

/**
 * OLTs the map cannot place, which the page must still account for. Defined as
 * the complement of mappedOlts so the two always partition the list: an OLT
 * with only one coordinate belongs here, and appears in neither twice.
 */
export function unmappedOlts(olts: Olt[] | undefined): Olt[] {
  return (olts ?? []).filter((olt) => !isMapped(olt));
}
