/** The shape this module needs from a Google geocoder result. */
export interface GeocodeResult {
  formatted_address?: string;
}

/**
 * The address to put in the form after a click on the map.
 *
 * Google returns its results most specific first, so the first one is the
 * street address rather than the province. Anything else — no results, a result
 * with no address — leaves the field empty for the operator to type, because a
 * wrong address is worse than a blank one when someone drives to it.
 */
export function addressFromGeocode(
  results: GeocodeResult[] | null | undefined,
): string {
  const first = (results ?? []).find(
    (result) =>
      typeof result.formatted_address === "string" &&
      result.formatted_address.trim() !== "",
  );
  return first?.formatted_address?.trim() ?? "";
}
