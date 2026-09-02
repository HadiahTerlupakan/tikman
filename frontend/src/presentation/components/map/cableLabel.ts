import type { CableSegment } from "./cableSegments";

/**
 * How long a cable is, in the words that say where the number came from.
 *
 * A traced path was measured; a straight line is only the gap between two
 * points, and a cable following poles is always longer. Saying so on the label
 * is what stops the smaller number being read as cable length.
 */
export function cableLengthLabel(segment: CableSegment): string {
  const metres = Math.round(segment.meters);
  const shown =
    metres >= 1000 ? `${(metres / 1000).toFixed(2)} km` : `${metres} m`;
  return segment.traced ? shown : `${shown} (garis lurus)`;
}

/** What kind of cable this is, in the operator's words. */
export function cableKindLabel(segment: CableSegment): string {
  return segment.kind === "feeder" ? "Feeder ke ODC" : "Distribusi ke ODP";
}
