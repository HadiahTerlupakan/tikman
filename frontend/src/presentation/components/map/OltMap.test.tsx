import type { ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, beforeEach, vi } from "vitest";
import { OltModel, OltProtocol, OltStatus, type Olt } from "@/domain/entities";
import { OltMap } from "./OltMap";

interface RecordedMapProps {
  mapId?: string;
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
  AdvancedMarker: ({
    title,
    onClick,
  }: {
    title: string;
    onClick: () => void;
  }) => (
    <button type="button" onClick={onClick}>
      {title}
    </button>
  ),
  InfoWindow: ({ children }: { children: ReactNode }) => (
    <div data-testid="info-window">{children}</div>
  ),
}));

function olt(overrides: Partial<Olt> & { id: string; name: string }): Olt {
  return {
    siteId: "s1",
    siteName: "Depok",
    ipAddress: "192.168.220.22",
    model: OltModel.ZTE_C320,
    preferredProtocol: OltProtocol.SSH,
    username: "admin",
    snmpCommunity: "public",
    sshPort: 22,
    telnetPort: 23,
    snmpPort: 161,
    status: OltStatus.ONLINE,
    ontCount: 0,
    lastSeen: null,
    rack: 0,
    shelf: 0,
    slot: 0,
    createdAt: "",
    updatedAt: "",
    ...overrides,
  };
}

describe("OltMap", () => {
  beforeEach(() => {
    mapProps = {};
  });

  it("draws a pin for every OLT that has coordinates", () => {
    render(
      <OltMap
        apiKey="AIzaTEST"
        olts={[
          olt({
            id: "o1",
            name: "OLT Depok",
            latitude: -6.4,
            longitude: 106.8,
          }),
          olt({
            id: "o2",
            name: "OLT Bekasi",
            latitude: -6.24,
            longitude: 107.0,
          }),
        ]}
      />,
    );

    expect(screen.getByRole("button", { name: "OLT Depok" })).toBeVisible();
    expect(screen.getByRole("button", { name: "OLT Bekasi" })).toBeVisible();
  });

  it("draws on the Cloud map the installation configured", () => {
    // Advanced markers render nothing without a map ID, so this is what
    // decides whether the map carries any pins at all.
    render(
      <OltMap
        apiKey="AIzaTEST"
        mapId="MAPTEST"
        olts={[olt({ id: "o1", name: "A", latitude: -6.4, longitude: 106.8 })]}
      />,
    );

    expect(mapProps.mapId).toBe("MAPTEST");
  });

  it("draws nothing for an OLT with no coordinates", () => {
    render(
      <OltMap
        apiKey="AIzaTEST"
        olts={[olt({ id: "o1", name: "OLT Cariu" })]}
      />,
    );

    expect(
      screen.queryByRole("button", { name: "OLT Cariu" }),
    ).not.toBeInTheDocument();
  });

  it("opens the map on Indonesia when nothing can be pinned", () => {
    // Centring on 0,0 would drop the operator into the Atlantic.
    render(<OltMap apiKey="AIzaTEST" olts={[olt({ id: "o1", name: "X" })]} />);

    expect(mapProps.defaultCenter).toEqual({ lat: -2.5, lng: 118 });
  });

  it("frames every pin rather than centring on the first", () => {
    // Depok and Bekasi are ~40km apart; a street-level centre on one of them
    // shows a single pin and reads as though only one OLT is mapped.
    render(
      <OltMap
        apiKey="AIzaTEST"
        olts={[
          olt({ id: "o1", name: "A", latitude: -6.4, longitude: 106.8 }),
          olt({ id: "o2", name: "B", latitude: -6.24, longitude: 107.0 }),
        ]}
      />,
    );

    expect(mapProps.defaultCenter).toBeUndefined();
    expect(mapProps.defaultBounds).toMatchObject({
      north: -6.24,
      south: -6.4,
      east: 107.0,
      west: 106.8,
    });
  });

  it("tells the operator what is at the pin they clicked", async () => {
    render(
      <OltMap
        apiKey="AIzaTEST"
        olts={[
          olt({
            id: "o1",
            name: "OLT Depok",
            siteName: "Depok",
            ipAddress: "192.168.220.22",
            status: OltStatus.ONLINE,
            ontCount: 199,
            latitude: -6.4,
            longitude: 106.8,
          }),
        ]}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "OLT Depok" }));

    const info = screen.getByTestId("info-window");
    expect(info).toHaveTextContent("OLT Depok");
    expect(info).toHaveTextContent("Site: Depok");
    expect(info).toHaveTextContent("192.168.220.22");
    expect(info).toHaveTextContent("online");
    expect(info).toHaveTextContent("199 ONTs");
  });

  it("counts a single ONT without saying 1 ONTs", async () => {
    render(
      <OltMap
        apiKey="AIzaTEST"
        olts={[
          olt({
            id: "o1",
            name: "Solo",
            ontCount: 1,
            latitude: -6.4,
            longitude: 106.8,
          }),
        ]}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Solo" }));

    expect(screen.getByTestId("info-window")).toHaveTextContent("1 ONT");
  });
});
