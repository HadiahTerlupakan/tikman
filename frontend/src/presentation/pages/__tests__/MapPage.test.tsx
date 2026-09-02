import { render, screen } from "@testing-library/react";
import { MemoryRouter } from "react-router-dom";
import { describe, expect, it, vi, beforeEach } from "vitest";
import MapPage from "../MapPage";

// MapPage links to /settings, and the unmapped panel links to /olts; both need
// a Router ancestor even in a unit test, or react-router-dom's <Link> throws.
const renderMapPage = () =>
  render(
    <MemoryRouter>
      <MapPage />
    </MemoryRouter>,
  );

const state = {
  key: undefined as string | undefined,
  olts: [] as unknown[],
  odcs: [] as unknown[],
  odps: [] as unknown[],
  feeds: [] as unknown[],
};

vi.mock("@/application/hooks", () => ({
  useGoogleMapsKey: () => ({ key: state.key, isLoading: false }),
  useOlts: () => ({ data: state.olts, isLoading: false }),
  useOdcs: () => ({ data: state.odcs }),
  useOdps: () => ({ data: state.odps }),
  useOdcFeeds: () => ({ data: state.feeds }),
  useSetCableRoute: () => ({ mutateAsync: vi.fn(), isPending: false }),
}));

// The map itself needs a Google API to draw anything; OltMap has its own test.
vi.mock("@/presentation/components/map/OltMap", () => ({
  OltMap: () => <div data-testid="olt-map" />,
}));

// The form reaches for sites, cabinets and OLTs of its own. Those belong to its
// test, not to the page's.
vi.mock("@/presentation/components/map/PlantFormModal", () => ({
  PlantFormModal: () => null,
}));

describe("MapPage", () => {
  beforeEach(() => {
    state.key = "AIzaSyTESTKEY123";
    state.olts = [];
    state.odcs = [];
    state.odps = [];
    state.feeds = [];
  });

  it("explains a missing key instead of rendering a broken map", () => {
    state.key = undefined;

    renderMapPage();

    expect(screen.getByText(/no google maps api key/i)).toBeInTheDocument();
    expect(screen.queryByTestId("olt-map")).not.toBeInTheDocument();
  });

  it("renders the map once a key is configured", () => {
    renderMapPage();

    expect(screen.getByTestId("olt-map")).toBeInTheDocument();
  });

  it("lists OLTs with no coordinates beside the map", () => {
    // This installation's OLTs have no coordinates yet, so this panel is the
    // first thing the operator will actually see on the page.
    state.olts = [
      {
        id: "o1",
        name: "OLT Cariu",
        siteName: "Cariu",
        ipAddress: "172.30.30.2",
      },
    ];

    renderMapPage();

    expect(screen.getByText("OLT Cariu")).toBeInTheDocument();
    expect(screen.getByText(/1 OLT has no coordinates/i)).toBeInTheDocument();
  });

  it("counts only the OLTs it can actually draw", () => {
    state.olts = [
      { id: "o1", name: "Mapped", latitude: -6.4, longitude: 106.8 },
      { id: "o2", name: "Unmapped" },
    ];

    renderMapPage();

    expect(
      screen.getByText(/1 OLT · 0 ODC · 0 ODP di peta/),
    ).toBeInTheDocument();
  });
});
