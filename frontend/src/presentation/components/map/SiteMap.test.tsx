import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, beforeEach, vi } from "vitest";
import { OltStatus, type Olt, type Site } from "@/domain/entities";
import { SiteMap } from "./SiteMap";

interface RecordedMapProps {
  defaultCenter?: { lat: number; lng: number };
  defaultZoom?: number;
  defaultBounds?: {
    north: number;
    south: number;
    east: number;
    west: number;
  };
}

let mapProps: RecordedMapProps;

// The real Map talks to Google's script, which cannot load under jsdom. These
// stands-in keep the camera props and the pin list observable, which is what
// the page's behaviour actually consists of.
vi.mock("@vis.gl/react-google-maps", () => ({
  APIProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  Map: ({ children, ...props }: RecordedMapProps & { children: ReactNode }) => {
    mapProps = props;
    return <div data-testid="map">{children}</div>;
  },
  Marker: ({ title, onClick }: { title: string; onClick: () => void }) => (
    <button type="button" onClick={onClick}>
      {title}
    </button>
  ),
  InfoWindow: ({ children }: { children: ReactNode }) => (
    <div data-testid="info-window">{children}</div>
  ),
}));

function site(overrides: Partial<Site> & { id: string; name: string }): Site {
  return {
    location: "",
    description: "",
    oltCount: 0,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

const depok = site({
  id: "depok",
  name: "Depok",
  location: "Jl. Margonda",
  latitude: -6.4025,
  longitude: 106.7942,
});
const bekasi = site({
  id: "bekasi",
  name: "Bekasi",
  latitude: -6.2383,
  longitude: 106.9756,
});
const gudang = site({ id: "gudang", name: "Gudang" });

function olt(id: string, siteId: string, status: OltStatus): Olt {
  return { id, siteId, status } as Olt;
}

describe("SiteMap", () => {
  beforeEach(() => {
    mapProps = {};
  });

  it("draws a pin for every mapped site and none for the rest", () => {
    render(<SiteMap apiKey="k" sites={[depok, gudang, bekasi]} olts={[]} />);

    expect(screen.getByRole("button", { name: "Depok" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Bekasi" })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: "Gudang" })).toBeNull();
  });

  it("frames every pin rather than opening on whichever came back first", () => {
    // Depok and Bekasi are ~40 km apart. Centring on one at street zoom shows
    // a single site and reads as though only one is mapped, and which one it
    // would be is decided by unordered Postgres rows.
    render(<SiteMap apiKey="k" sites={[depok, bekasi]} olts={[]} />);

    expect(mapProps.defaultCenter).toBeUndefined();
    expect(mapProps.defaultBounds).toMatchObject({
      north: -6.2383,
      south: -6.4025,
      east: 106.9756,
      west: 106.7942,
    });
  });

  it("opens a single site at a readable zoom instead of the maximum", () => {
    render(<SiteMap apiKey="k" sites={[depok, gudang]} olts={[]} />);

    expect(mapProps.defaultCenter).toEqual({ lat: -6.4025, lng: 106.7942 });
    expect(mapProps.defaultZoom).toBe(14);
    expect(mapProps.defaultBounds).toBeUndefined();
  });

  it("falls back to Indonesia when nothing is mapped", () => {
    render(<SiteMap apiKey="k" sites={[gudang]} olts={[]} />);

    expect(mapProps.defaultCenter).toEqual({ lat: -2.5, lng: 118 });
    expect(mapProps.defaultZoom).toBe(4);
  });

  it("reports how many of a site's OLTs are online", async () => {
    render(
      <SiteMap
        apiKey="k"
        sites={[depok, bekasi]}
        olts={[
          olt("a", "depok", OltStatus.ONLINE),
          olt("b", "depok", OltStatus.OFFLINE),
          olt("c", "bekasi", OltStatus.ONLINE),
        ]}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Depok" }));

    expect(screen.getByTestId("info-window")).toHaveTextContent(
      "1 of 2 OLTs online",
    );
    expect(screen.getByTestId("info-window")).toHaveTextContent("Jl. Margonda");
  });

  it("says so when a site has no OLTs, rather than showing 0 of 0", async () => {
    render(
      <SiteMap
        apiKey="k"
        sites={[depok, bekasi]}
        olts={[olt("c", "bekasi", OltStatus.ONLINE)]}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Depok" }));

    expect(screen.getByTestId("info-window")).toHaveTextContent(
      "No OLTs at this site",
    );
  });
});
