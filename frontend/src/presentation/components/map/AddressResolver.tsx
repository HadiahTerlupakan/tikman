import { useEffect } from "react";
import { APIProvider, useMapsLibrary } from "@vis.gl/react-google-maps";
import { useGoogleMapsKey } from "@/application/hooks";
import { addressFromGeocode } from "./reverseGeocode";
import type { Coordinates } from "./plantForm";

interface AddressResolverProps {
  coordinates?: Coordinates;
  onResolved: (address: string) => void;
}

/**
 * Turns the point the operator clicked into a street address.
 *
 * Renders nothing: it exists so the form can be filled in rather than typed.
 * Its own APIProvider, the way AddressAutocomplete has one, because the modal
 * sits outside the map's.
 *
 * Needs the Geocoding API enabled on the Maps key. When it is not — or when
 * Google finds nothing — the address field is simply left for the operator to
 * type, since a wrong address is worse than a blank one to drive to.
 */
export function AddressResolver({
  coordinates,
  onResolved,
}: AddressResolverProps) {
  const { key } = useGoogleMapsKey();
  if (!key || !coordinates) {
    return null;
  }
  return (
    <APIProvider apiKey={key} libraries={["geocoding"]}>
      <Resolver coordinates={coordinates} onResolved={onResolved} />
    </APIProvider>
  );
}

function Resolver({
  coordinates,
  onResolved,
}: {
  coordinates: Coordinates;
  onResolved: (address: string) => void;
}) {
  const geocoding = useMapsLibrary("geocoding");

  useEffect(() => {
    if (!geocoding) {
      return;
    }
    let cancelled = false;
    const geocoder = new geocoding.Geocoder();
    geocoder
      .geocode({
        location: { lat: coordinates.latitude, lng: coordinates.longitude },
      })
      .then((response) => {
        if (!cancelled) {
          onResolved(addressFromGeocode(response.results));
        }
      })
      .catch(() => {
        // The key may not carry the Geocoding API. Leaving the field to the
        // operator is the whole fallback; there is nothing to report.
      });
    return () => {
      cancelled = true;
    };
  }, [geocoding, coordinates, onResolved]);

  return null;
}
