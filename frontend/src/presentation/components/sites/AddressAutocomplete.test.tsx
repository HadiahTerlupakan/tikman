import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import { AddressAutocomplete } from "./AddressAutocomplete";

const mapsKey: { key?: string; isLoading: boolean } = {
  key: undefined,
  isLoading: false,
};

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => mapsKey,
}));

// Without this, APIProvider would try to fetch Google's script under jsdom.
vi.mock("@vis.gl/react-google-maps", () => ({
  APIProvider: ({ children }: { children: React.ReactNode }) => <>{children}</>,
  useMapsLibrary: () => null,
}));

describe("AddressAutocomplete", () => {
  beforeEach(() => {
    mapsKey.key = undefined;
    mapsKey.isLoading = false;
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
});
