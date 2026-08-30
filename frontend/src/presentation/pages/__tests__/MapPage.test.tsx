import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi, beforeEach } from "vitest";
import MapPage from "../MapPage";

// MapPage links to /settings and to the unmapped-sites panel's /sites link,
// which need a Router ancestor even in a unit test — without it
// react-router-dom's <Link> throws on render.
const renderMapPage = () =>
  render(
    <MemoryRouter>
      <MapPage />
    </MemoryRouter>,
  );

const state = {
  key: undefined as string | undefined,
  sites: [] as unknown[],
};

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => ({ key: state.key, isLoading: false }),
  useSites: () => ({ data: state.sites, isLoading: false }),
  useOlts: () => ({ data: [], isLoading: false }),
}));

// MapPage also imports mappedSites/unmappedSites from this module (see the
// comment there on why it isn't imported from the barrel); only the
// component itself needs replacing, so keep the real helpers via
// importOriginal instead of dropping them.
vi.mock("@/presentation/components/map/SiteMap", async (importOriginal) => ({
  ...(await importOriginal<
    typeof import("@/presentation/components/map/SiteMap")
  >()),
  SiteMap: () => <div data-testid="site-map" />,
}));

describe("MapPage", () => {
  beforeEach(() => {
    state.key = "AIzaSyTESTKEY123";
    state.sites = [];
  });

  it("explains a missing key instead of rendering a broken map", () => {
    state.key = undefined;

    renderMapPage();

    expect(screen.getByText(/no google maps api key/i)).toBeInTheDocument();
    expect(screen.queryByTestId("site-map")).not.toBeInTheDocument();
  });

  it("renders the map once a key is configured", () => {
    renderMapPage();

    expect(screen.getByTestId("site-map")).toBeInTheDocument();
  });

  it("lists sites with no coordinates beside the map", () => {
    state.sites = [
      {
        id: "s1",
        name: "Gudang",
        location: "Belakang kantor",
        description: "",
        oltCount: 1,
        createdAt: "",
        updatedAt: "",
      },
    ];

    renderMapPage();

    expect(screen.getByText("Gudang")).toBeInTheDocument();
  });
});
