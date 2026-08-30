import { useEffect, useRef } from "react";
import { Input } from "antd";
import { APIProvider, useMapsLibrary } from "@vis.gl/react-google-maps";
import { useGoogleMapsKey } from "@/application/hooks";

export interface ResolvedPlace {
  address: string;
  latitude: number;
  longitude: number;
}

interface AddressAutocompleteProps {
  // Injected by the Form.Item that wraps this field; absent when it is used
  // outside a form.
  value?: string;
  onChange?: (value: string) => void;
  onResolved: (place: ResolvedPlace) => void;
}

/**
 * An address field that suggests real places when a Maps key is configured and
 * is an ordinary text input when one is not.
 *
 * Suggestions are a convenience, never a requirement: Places does not know a
 * POP down a gang or a tower in a field, and if Google were the only way to set
 * a location those sites could never be mapped at all. Whatever happens here,
 * the operator can still type an address and the coordinate fields beside it.
 */
export function AddressAutocomplete(props: AddressAutocompleteProps) {
  const { key } = useGoogleMapsKey();

  if (!key) {
    return <PlainAddressInput {...props} />;
  }

  return (
    <APIProvider apiKey={key} libraries={["places"]}>
      <SuggestingAddressInput {...props} />
    </APIProvider>
  );
}

function PlainAddressInput({ value, onChange }: AddressAutocompleteProps) {
  return (
    <Input
      value={value ?? ""}
      placeholder="Address"
      onChange={(event) => onChange?.(event.target.value)}
    />
  );
}

function SuggestingAddressInput({
  value,
  onChange,
  onResolved,
}: AddressAutocompleteProps) {
  // Null until the library finishes loading, and it stays null if the script
  // never arrives — which leaves a working text field rather than an error the
  // operator cannot act on mid-form.
  const places = useMapsLibrary("places");
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const container = containerRef.current;
    if (!places || !container) {
      return;
    }

    // google.maps.places.Autocomplete is legacy: as of March 1st 2025 Google
    // no longer issues it to new Cloud projects, which every fresh TikMan
    // install would be. PlaceAutocompleteElement is the surface new keys
    // actually get.
    const element = new places.PlaceAutocompleteElement({
      value: value ?? "",
    });
    element.placeholder = "Start typing an address";
    container.appendChild(element);

    // The element owns its own <input> in a shadow root; there is no
    // gmp-* event for free typing, but native "input" events are composed
    // and cross the shadow boundary, so this still sees every keystroke.
    const handleInput = () => onChange?.(element.value);
    element.addEventListener("input", handleInput);

    // Typed as the base Event, not PlacePredictionSelectEvent: @types/google.maps
    // only widens addEventListener's overload for "gmp-select", and
    // removeEventListener still only knows HTMLElement's built-in event
    // names, so a listener typed to the narrower event fails there.
    const handleSelect = (event: Event) => {
      const { placePrediction } =
        event as google.maps.places.PlacePredictionSelectEvent;
      void placePrediction
        .toPlace()
        .fetchFields({ fields: ["formattedAddress", "location"] })
        .then(({ place }) => {
          const location = place.location;
          if (!location) {
            return;
          }
          const address = place.formattedAddress ?? element.value;
          onChange?.(address);
          onResolved({
            address,
            latitude: location.lat(),
            longitude: location.lng(),
          });
        })
        // A failed fetch (quota, network) leaves the text the operator
        // already typed in place; manual latitude/longitude are still there
        // to fall back on, so there is nothing more to do here.
        .catch(() => undefined);
    };
    element.addEventListener("gmp-select", handleSelect);

    return () => {
      element.removeEventListener("input", handleInput);
      element.removeEventListener("gmp-select", handleSelect);
      container.removeChild(element);
    };
    // value only seeds the element when it is created; re-running this on
    // every keystroke would tear the widget down mid-type and drop focus.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [places, onChange, onResolved]);

  // Before the Places script has loaded, this is the same plain field as the
  // no-key path, so the operator can start typing immediately either way.
  if (!places) {
    return (
      <Input
        value={value ?? ""}
        placeholder="Start typing an address"
        onChange={(event) => onChange?.(event.target.value)}
      />
    );
  }

  return <div ref={containerRef} />;
}
