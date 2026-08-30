import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { Form, type FormInstance } from "antd";
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
  placeholder = "";
  // The real element owns its <input> inside a shadow root and the component
  // depends on native "input" events being composed to cross that boundary.
  // Reproducing the shadow root is what lets a keystroke in a test travel the
  // same path it takes in a browser.
  readonly innerInput: HTMLInputElement;

  constructor(options?: { value?: string }) {
    super();
    this.innerInput = document.createElement("input");
    this.innerInput.value = options?.value ?? "";
    this.attachShadow({ mode: "open" }).appendChild(this.innerInput);
    FakePlaceAutocompleteElement.instances.push(this);
  }

  get value(): string {
    return this.innerInput.value;
  }

  set value(next: string) {
    this.innerInput.value = next;
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

// Renders the field the way SiteModal does — wrapped in a real Form.Item, so
// the value/onChange antd injects behave exactly as they do in the app. A
// hand-passed vi.fn() onChange is stable and cannot reproduce that.
let formUnderTest: FormInstance;

function FormHost({ onResolved }: { onResolved: () => void }) {
  const [form] = Form.useForm();
  formUnderTest = form;
  return (
    <Form form={form}>
      <Form.Item name="location">
        <AddressAutocomplete onResolved={onResolved} />
      </Form.Item>
    </Form>
  );
}

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

  it("survives typing inside a Form.Item without being rebuilt", async () => {
    // rc-field-form hands Form.Item a fresh onChange closure on every render
    // and antd's MemoInput treats every function prop as unchanged, so only a
    // value change gets through — which is exactly what a keystroke is. With
    // onChange in the effect's dependencies each character destroyed the
    // element being typed into, focus fell to <body>, and the operator could
    // never enter more than one character.
    mapsKey.key = "AIzaSyTESTKEY123";
    placesLibrary = { PlaceAutocompleteElement: FakePlaceAutocompleteElement };

    render(<FormHost onResolved={vi.fn()} />);

    const [element] = await waitFor(() => {
      expect(FakePlaceAutocompleteElement.instances).toHaveLength(1);
      return FakePlaceAutocompleteElement.instances;
    });

    await userEvent.type(element.innerInput, "Jl");

    expect(FakePlaceAutocompleteElement.instances).toHaveLength(1);
    expect(element.isConnected).toBe(true);
    expect(element.shadowRoot?.activeElement).toBe(element.innerInput);
    expect(formUnderTest.getFieldValue("location")).toBe("Jl");
  });
});
