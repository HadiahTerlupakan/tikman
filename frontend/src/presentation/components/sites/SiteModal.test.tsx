import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { SiteModal } from "./SiteModal";
import type { Site } from "@/domain/entities";

// SiteModal pulls in AddressAutocomplete, which calls the real
// useGoogleMapsKey. These tests are about manual entry, so there is no Maps
// key and no QueryClientProvider to serve one from.
vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => ({ key: undefined, isLoading: false }),
}));

describe("SiteModal", () => {
  it("saves a site typed with coordinates by hand", async () => {
    // Places does not know a POP down a gang or a tower in a field, which is a
    // large share of these sites. Manual entry is the path that always works.
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.type(screen.getByLabelText("Longitude"), "106.7942");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({
        name: "Gudang",
        latitude: -6.4025,
        longitude: 106.7942,
      }),
    );
  });

  it("saves a site with no coordinates at all", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Gudang" }),
    );
    expect(onSubmit.mock.calls[0][0].latitude).toBeUndefined();
  });

  it("refuses half a coordinate and says why", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal open onClose={vi.fn()} onSubmit={onSubmit} loading={false} />,
    );

    await userEvent.type(screen.getByLabelText("Site Name"), "Gudang");
    await userEvent.type(screen.getByLabelText("Latitude"), "-6.4025");
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(await screen.findByText(/together/i)).toBeInTheDocument();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  const baseSite: Site = {
    id: "1",
    name: "Gudang",
    location: "Jl. Margonda",
    description: "",
    oltCount: 0,
    createdAt: "2026-01-01T00:00:00Z",
    updatedAt: "2026-01-01T00:00:00Z",
  };

  it("populates and round-trips an existing site's coordinates", async () => {
    const onSubmit = vi.fn();
    const site: Site = { ...baseSite, latitude: -6.4025, longitude: 106.7942 };
    render(
      <SiteModal
        open
        site={site}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        loading={false}
      />,
    );

    expect(await screen.findByLabelText("Latitude")).toHaveValue("-6.4025");
    expect(screen.getByLabelText("Longitude")).toHaveValue("106.7942");

    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ latitude: -6.4025, longitude: 106.7942 }),
    );
  });

  it("asks the API to clear the pin when both fields are emptied", async () => {
    // A JSON null and an omitted key look identical to the API, so sending
    // nothing would report success and leave the wrong pin on the map.
    const onSubmit = vi.fn();
    const site: Site = { ...baseSite, latitude: -6.4025, longitude: 106.7942 };
    render(
      <SiteModal
        open
        site={site}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        loading={false}
      />,
    );

    await userEvent.clear(await screen.findByLabelText("Latitude"));
    await userEvent.clear(screen.getByLabelText("Longitude"));
    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ clearCoordinates: true }),
    );
    expect(onSubmit.mock.calls[0][0].latitude).toBeUndefined();
  });

  it("does not ask to clear a pin on a site that never had one", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal
        open
        site={baseSite}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        loading={false}
      />,
    );

    await userEvent.click(await screen.findByRole("button", { name: /ok/i }));

    expect(onSubmit.mock.calls[0][0].clearCoordinates).toBeUndefined();
  });

  it("leaves coordinates empty for a site that has none, and still saves", async () => {
    const onSubmit = vi.fn();
    render(
      <SiteModal
        open
        site={baseSite}
        onClose={vi.fn()}
        onSubmit={onSubmit}
        loading={false}
      />,
    );

    expect(await screen.findByLabelText("Latitude")).toHaveValue("");
    expect(screen.getByLabelText("Longitude")).toHaveValue("");

    await userEvent.click(screen.getByRole("button", { name: /ok/i }));

    expect(onSubmit).toHaveBeenCalledWith(
      expect.objectContaining({ name: "Gudang" }),
    );
    expect(onSubmit.mock.calls[0][0].latitude).toBeUndefined();
  });
});
