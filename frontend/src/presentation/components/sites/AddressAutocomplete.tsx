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

const PLAIN_PLACEHOLDER = "Address";
const SUGGESTING_PLACEHOLDER = "Start typing an address";

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
    return (
      <PlainAddressInput
        value={props.value}
        onChange={props.onChange}
        placeholder={PLAIN_PLACEHOLDER}
      />
    );
  }

  return (
    <APIProvider apiKey={key} libraries={["places"]}>
      <SuggestingAddressInput {...props} />
    </APIProvider>
  );
}

function PlainAddressInput({
  value,
  onChange,
  placeholder,
}: {
  value?: string;
  onChange?: (value: string) => void;
  placeholder: string;
}) {
  return (
    <Input
      value={value ?? ""}
      placeholder={placeholder}
      onChange={(event) => onChange?.(event.target.value)}
    />
  );
}

/**
 * Fetches the full place for a gmp-select event and reports it, unless
 * Google could not geocode the suggestion — a selection with no location
 * must never resolve to 0,0 (the Gulf of Guinea), which would look like a
 * real, deliberately placed site.
 *
 * Typed as the base Event, not PlacePredictionSelectEvent: @types/google.maps
 * only widens addEventListener's overload for "gmp-select", and
 * removeEventListener still only knows HTMLElement's built-in event names, so
 * a listener typed to the narrower event fails there.
 */
function resolvePlaceSelection(
  event: Event,
  element: google.maps.places.PlaceAutocompleteElement,
  onChange: ((value: string) => void) | undefined,
  onResolved: (place: ResolvedPlace) => void,
): Promise<void> {
  const { placePrediction } =
    event as google.maps.places.PlacePredictionSelectEvent;
  return (
    placePrediction
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
      .catch(() => undefined)
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

  // Form.Item rebuilds onChange on every render and antd's MemoInput treats
  // all function props as equal, so a keystroke re-renders this with a fresh
  // onChange identity. Reaching the callbacks through refs keeps them out of
  // the effect's dependencies: with them in, every character tore down the
  // element being typed into, dropped focus to <body>, and the suggestion
  // dropdown could never hold more than one character.
  const onChangeRef = useRef(onChange);
  const onResolvedRef = useRef(onResolved);
  useEffect(() => {
    onChangeRef.current = onChange;
    onResolvedRef.current = onResolved;
  });

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
    element.placeholder = SUGGESTING_PLACEHOLDER;
    container.appendChild(element);

    // The element owns its own <input> in a shadow root; there is no
    // gmp-* event for free typing, but native "input" events are composed
    // and cross the shadow boundary, so this still sees every keystroke.
    const handleInput = () => onChangeRef.current?.(element.value);
    element.addEventListener("input", handleInput);

    const handleSelect = (event: Event) => {
      void resolvePlaceSelection(event, element, onChangeRef.current, (place) =>
        onResolvedRef.current(place),
      );
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
  }, [places]);

  // Before the Places script has loaded, this is the same plain field as the
  // no-key path, so the operator can start typing immediately either way.
  if (!places) {
    return (
      <PlainAddressInput
        value={value}
        onChange={onChange}
        placeholder={SUGGESTING_PLACEHOLDER}
      />
    );
  }

  return <div ref={containerRef} />;
}
