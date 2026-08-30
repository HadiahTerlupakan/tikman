import type { Site } from "@/domain/entities";

/** A site the map can actually draw, with both coordinates known to be present. */
export type MappedSite = Site & { latitude: number; longitude: number };

function isMapped(site: Site): site is MappedSite {
  return (
    typeof site.latitude === "number" && typeof site.longitude === "number"
  );
}

/** Sites carrying both coordinates, which are the only ones that can be drawn. */
export function mappedSites(sites: Site[] | undefined): MappedSite[] {
  return (sites ?? []).filter(isMapped);
}

/**
 * Sites the map cannot place, which the page must still account for. Defined as
 * the complement of mappedSites so the two always partition the list: a site
 * with only one coordinate belongs here, and appears in neither twice.
 */
export function unmappedSites(sites: Site[] | undefined): Site[] {
  return (sites ?? []).filter((site) => !isMapped(site));
}
