import { Ont } from "@/domain/entities/Ont";

/** Filters ONTs by serial number or name, case-insensitive. */
export function filterOntsByQuery(onts: Ont[], query: string): Ont[] {
  const q = query.trim().toLowerCase();
  if (!q) return onts;
  return onts.filter(
    (ont) =>
      ont.serialNumber.toLowerCase().includes(q) ||
      ont.name.toLowerCase().includes(q),
  );
}

/**
 * Describes how many ONTs the page is showing. The list endpoint caps a page at
 * 500, and the search box filters what was loaded rather than asking the
 * server, so an OLT with more than that has ONTs the search cannot reach. Say
 * so instead of reporting the loaded count as the total.
 */
export function describeOntCoverage(
  shown: number,
  matching: number,
  loaded: number,
  available: number,
): string {
  const base = `Showing ${shown} of ${matching} ONTs`;
  if (available <= loaded) return base;
  return `${base} — only the first ${loaded} of ${available} are loaded, so narrow the filter to reach the rest`;
}
