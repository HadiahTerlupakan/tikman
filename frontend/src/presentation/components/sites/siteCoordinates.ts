/** Reads a decimal degree, or null when the text is not one. */
export function parseCoordinate(input: string): number | null {
  const trimmed = input.trim();
  if (trimmed === "") {
    return null;
  }
  // Number("") is 0 and Number("6,4025") is NaN; the explicit pattern keeps a
  // comma decimal from silently becoming a different place.
  if (!/^[+-]?\d+(\.\d+)?$/.test(trimmed)) {
    return null;
  }
  return Number(trimmed);
}

/**
 * Returns the reason a coordinate pair cannot be saved, or null when it can.
 * Both empty is allowed: not every site can be placed, and a site must never
 * become unsavable because a location could not be resolved.
 */
export function coordinateError(
  latitude: string,
  longitude: string,
): string | null {
  const hasLatitude = latitude.trim() !== "";
  const hasLongitude = longitude.trim() !== "";

  if (!hasLatitude && !hasLongitude) {
    return null;
  }
  if (hasLatitude !== hasLongitude) {
    return "Latitude and longitude must be given together, or both left empty";
  }

  const lat = parseCoordinate(latitude);
  const lng = parseCoordinate(longitude);
  if (lat === null || lng === null) {
    return "Coordinates must be a number, for example -6.4025";
  }
  if (lat < -90 || lat > 90) {
    return "Latitude must be between -90 and 90";
  }
  if (lng < -180 || lng > 180) {
    return "Longitude must be between -180 and 180";
  }
  return null;
}
