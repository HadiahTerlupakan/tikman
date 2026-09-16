import type { ComponentProps, ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { MapCanvas, edgePath } from "./MapCanvas";

interface RecordedMapProps {
  onClick?: (event: {
    detail: { latLng: { lat: number; lng: number } | null };
  }) => void;
}

let mapProps: RecordedMapProps;

// The real Map talks to Google's script, which cannot load under jsdom. These
// stand-ins keep the click handler and the drawn paths observable, which is
// what the interaction actually consists of.
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
  Polyline: ({ path }: { path: { lat: number; lng: number }[] }) => (
    <div data-testid="polyline" data-points={path.length} />
  ),
}));

function node(
  overrides: Partial<MappingNode> & { nodeId: string },
): MappingNode {
  return {
    type: "odp",
    name: overrides.nodeId,
    latitude: -6.2,
    longitude: 106.8,
    capacity: 0,
    splitter: "",
    pppoe: "",
    serialNumber: "",
    notes: "",
    ...overrides,
  };
}

function edge(
  overrides: Partial<MappingEdge> & {
    edgeId: string;
    source: string;
    target: string;
  },
): MappingEdge {
  return {
    fiberType: "drop",
    distance: 0,
    waypoints: [],
    notes: "",
    ...overrides,
  };
}

const noop = () => {};

const defaultProps: ComponentProps<typeof MapCanvas> = {
  nodes: [],
  edges: [],
  draft: [],
  placing: undefined,
  apiKey: "AIzaTEST",
  onDrop: noop,
  onNodeClick: noop,
};

function renderCanvas(
  overrides: Partial<ComponentProps<typeof MapCanvas>> = {},
) {
  const result = render(<MapCanvas {...defaultProps} {...overrides} />);
  return {
    ...result,
    rerenderWith: (next: Partial<ComponentProps<typeof MapCanvas>>) =>
      result.rerender(<MapCanvas {...defaultProps} {...next} />),
  };
}

describe("edgePath", () => {
  const source = node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 });
  const target = node({
    nodeId: "ODP-01",
    latitude: -6.21,
    longitude: 106.81,
  });
  const nodesById = new Map([
    [source.nodeId, source],
    [target.nodeId, target],
  ]);

  it("spans from the source, through every traced corner, to the target", () => {
    const path = edgePath(
      edge({
        edgeId: "E1",
        source: "ODC-01",
        target: "ODP-01",
        waypoints: [{ lat: -6.205, lng: 106.805 }],
      }),
      nodesById,
    );

    expect(path).toEqual([
      { lat: -6.2, lng: 106.8 },
      { lat: -6.205, lng: 106.805 },
      { lat: -6.21, lng: 106.81 },
    ]);
  });

  it("is skipped when the target has not been named yet", () => {
    const path = edgePath(
      edge({ edgeId: "E2", source: "ODC-01", target: "GHOST-404" }),
      nodesById,
    );

    expect(path).toBeUndefined();
  });

  it("is skipped when the source has been removed from the map", () => {
    const path = edgePath(
      edge({ edgeId: "E3", source: "GHOST-404", target: "ODP-01" }),
      nodesById,
    );

    expect(path).toBeUndefined();
  });
});

describe("MapCanvas", () => {
  beforeEach(() => {
    mapProps = {};
  });

  it("draws a marker per node and identifies a click by its mapping code, not its database id", async () => {
    const onNodeClick = vi.fn();
    renderCanvas({
      nodes: [
        node({
          nodeId: "ODP-07",
          id: "11111111-1111-1111-1111-111111111111",
          name: "ODP Tujuh",
        }),
      ],
      onNodeClick,
    });

    await userEvent.click(screen.getByRole("button", { name: "ODP Tujuh" }));

    expect(onNodeClick).toHaveBeenCalledWith("ODP-07");
  });

  it("renders the cable whose ends are both named, and leaves the rest of the map standing when another edge's target has vanished", () => {
    renderCanvas({
      nodes: [
        node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 }),
        node({ nodeId: "ODP-01", latitude: -6.21, longitude: 106.81 }),
      ],
      edges: [
        edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" }),
        edge({ edgeId: "E2", source: "ODC-01", target: "GHOST-404" }),
      ],
    });

    expect(screen.getAllByTestId("polyline")).toHaveLength(1);
    expect(screen.getByRole("button", { name: "ODC-01" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ODP-01" })).toBeInTheDocument();
  });

  it("drops a point at the tapped coordinate only while something is being placed", () => {
    const onDrop = vi.fn();
    const { rerenderWith } = renderCanvas({ onDrop });

    mapProps.onClick?.({ detail: { latLng: { lat: -6.3, lng: 106.9 } } });
    expect(onDrop).not.toHaveBeenCalled();

    rerenderWith({ onDrop, placing: "odp" });
    mapProps.onClick?.({ detail: { latLng: { lat: -6.3, lng: 106.9 } } });

    expect(onDrop).toHaveBeenCalledWith({ lat: -6.3, lng: 106.9 });
  });

  it("draws the traced path only once it has a second point to connect", () => {
    const { rerenderWith } = renderCanvas({
      draft: [{ lat: -6.2, lng: 106.8 }],
    });

    expect(screen.queryAllByTestId("polyline")).toHaveLength(0);

    rerenderWith({
      draft: [
        { lat: -6.2, lng: 106.8 },
        { lat: -6.21, lng: 106.81 },
      ],
    });

    expect(screen.getAllByTestId("polyline")).toHaveLength(1);
  });
});
