import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { AddressAutocomplete } from "./AddressAutocomplete";

const mapsKey: { key?: string; isLoading: boolean } = {
  key: undefined,
  isLoading: false,
};

// A stand-in for google.maps.places.PlaceAutocompleteElement: a real custom
// element, because the component constructs one with `new` and calls
// addEventListener/removeEventListener on it directly. Browsers (and jsdom)
// refuse `new` on an HTMLElement subclass that was never registered, so it
// has to go through customElements.define like the real class does.
class FakePlaceAutocompleteElement extends HTMLElement {
  static instances: FakePlaceAutocompleteElement[] = [];
  value: string;
  placeholder = "";

  constructor(options?: { value?: string }) {
    super();
    this.value = options?.value ?? "";
    FakePlaceAutocompleteElement.instances.push(this);
  }
}
if (!customElements.get("fake-place-autocomplete-element")) {
  customElements.define(
    "fake-place-autocomplete-element",
    FakePlaceAutocompleteElement,
  );
}

interface FakePlace {
  formattedAddress?: string;
  location: { lat: () => number; lng: () => number } | null;
}

// Dispatches a gmp-select event on `element` carrying a placePrediction whose
// toPlace().fetchFields() resolves to `place`, mirroring the real
// PlacePredictionSelectEvent shape the component reads.
function selectPlace(element: FakePlaceAutocompleteElement, place: FakePlace) {
  const fetchFields = () => Promise.resolve({ place });
  const placePrediction = { toPlace: () => ({ fetchFields }) };
  const event = new Event("gmp-select") as Event & {
    placePrediction: typeof placePrediction;
  };
  event.placePrediction = placePrediction;
  element.dispatchEvent(event);
  return fetchFields();
}

let placesLibrary: {
  PlaceAutocompleteElement: typeof FakePlaceAutocompleteElement;
} | null = null;

// Without this, APIProvider would try to fetch Google's script under jsdom.
vi.mock("@vis.gl/react-google-maps", () => ({
  APIProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMapsLibrary: () => placesLibrary,
}));

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => mapsKey,
}));

describe("AddressAutocomplete", () => {
  beforeEach(() => {
    mapsKey.key = undefined;
    mapsKey.isLoading = false;
    placesLibrary = null;
    FakePlaceAutocompleteElement.instances = [];
  });

  it("is a plain text field when no Maps key is configured", async () => {
    // The form must not break, nag, or block saving because a credential is
    // missing.
    const onChange = vi.fn();
    render(
      <AddressAutocomplete value="" onChange={onChange} onResolved={vi.fn()} />,
    );

    await userEvent.type(screen.getByRole("textbox"), "Jl. Margonda");

    expect(onChange).toHaveBeenCalled();
    expect(screen.queryByText(/error/i)).not.toBeInTheDocument();
  });

  it("keeps what the operator typed when the key is present", async () => {
    mapsKey.key = "AIzaSyTESTKEY123";
    const onChange = vi.fn();

    render(
      <AddressAutocomplete value="" onChange={onChange} onResolved={vi.fn()} />,
    );

    await userEvent.type(screen.getByRole("textbox"), "Jl");

    expect(onChange).toHaveBeenCalled();
  });

  it("resolves a selected suggestion into an address and coordinates", async () => {
    mapsKey.key = "AIzaSyTESTKEY123";
    placesLibrary = { PlaceAutocompleteElement: FakePlaceAutocompleteElement };
    const onChange = vi.fn();
    const onResolved = vi.fn();

    render(
      <AddressAutocomplete
        value=""
        onChange={onChange}
        onResolved={onResolved}
      />,
    );

    const [element] = await waitFor(() => {
      expect(FakePlaceAutocompleteElement.instances).toHaveLength(1);
      return FakePlaceAutocompleteElement.instances;
    });

    await selectPlace(element, {
      formattedAddress: "Jl. Margonda, Depok",
      location: { lat: () => -6.4025, lng: () => 106.7942 },
    });

    await waitFor(() => {
      expect(onResolved).toHaveBeenCalledWith({
        address: "Jl. Margonda, Depok",
        latitude: -6.4025,
        longitude: 106.7942,
      });
    });
    expect(onChange).toHaveBeenCalledWith("Jl. Margonda, Depok");
  });

  it("does not resolve a suggestion Google could not geocode", async () => {
    // A place with no location must never turn into 0,0 — the Gulf of
    // Guinea — since that would look like a real, deliberately placed site.
    mapsKey.key = "AIzaSyTESTKEY123";
    placesLibrary = { PlaceAutocompleteElement: FakePlaceAutocompleteElement };
    const onResolved = vi.fn();

    render(
      <AddressAutocomplete
        value=""
        onChange={vi.fn()}
        onResolved={onResolved}
      />,
    );

    const [element] = await waitFor(() => {
      expect(FakePlaceAutocompleteElement.instances).toHaveLength(1);
      return FakePlaceAutocompleteElement.instances;
    });

    await selectPlace(element, {
      formattedAddress: "Somewhere Google can't place",
      location: null,
    });
    // Let the component's own .then() continuation run before asserting.
    await Promise.resolve();
    await Promise.resolve();

    expect(onResolved).not.toHaveBeenCalled();
  });
});
