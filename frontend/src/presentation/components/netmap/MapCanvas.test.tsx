import type { ComponentProps, ReactNode } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi, beforeEach } from "vitest";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { MapCanvas } from "./MapCanvas";

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
  Polyline: ({
    path,
    strokeColor,
  }: {
    path: { lat: number; lng: number }[];
    strokeColor?: string;
  }) => (
    <div
      data-testid="polyline"
      data-points={path.length}
      data-stroke-color={strokeColor}
    />
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

  it("renders every cable whose ends are named, and does not stop at the one in between whose target has vanished", () => {
    renderCanvas({
      nodes: [
        node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 }),
        node({ nodeId: "ODP-01", latitude: -6.21, longitude: 106.81 }),
        node({ nodeId: "ONT-09", latitude: -6.22, longitude: 106.82 }),
      ],
      // The dangling edge sits between two valid ones on purpose: `.map()`
      // has no early exit today, but a future rewrite to a loop that returns
      // or breaks on the first unresolved edge would still pass a fixture
      // where the only valid edge comes first. Putting a valid edge after
      // the ghost is what would catch that.
      edges: [
        edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" }),
        edge({ edgeId: "E2", source: "ODC-01", target: "GHOST-404" }),
        edge({ edgeId: "E3", source: "ODP-01", target: "ONT-09" }),
      ],
    });

    expect(screen.getAllByTestId("polyline")).toHaveLength(2);
    expect(screen.getByRole("button", { name: "ODC-01" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ODP-01" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "ONT-09" })).toBeInTheDocument();
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

  // useCableDraw.points holds only the tapped corners, never the node the
  // trace started from — drawing `draft` alone put the green line's start at
  // the first corner while the saved cable (edgePath) always starts at the
  // source node. Left alone, that is the opposite of what gets saved.
  it("starts the in-progress line at the source node, not its first tapped corner", () => {
    renderCanvas({
      nodes: [node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 })],
      fromNodeId: "ODC-01",
      draft: [
        { lat: -6.21, lng: 106.81 },
        { lat: -6.22, lng: 106.82 },
      ],
    });

    expect(screen.getByTestId("polyline")).toHaveAttribute("data-points", "3");
  });

  // The old route has to stay visible and tell apart from the new line being
  // traced (already true: saved edges are amber, the draft is green) — but it
  // must also tell apart from every *other* saved cable, or a technician
  // cannot see which one they are correcting on a map with several cables on
  // it.
  it("draws the cable being redrawn in a colour distinct from an ordinary saved cable", () => {
    const fixture = {
      nodes: [
        node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 }),
        node({ nodeId: "ODP-01", latitude: -6.21, longitude: 106.81 }),
      ],
      edges: [edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" })],
    };
    const { rerenderWith } = renderCanvas(fixture);
    const ordinaryColor = screen
      .getByTestId("polyline")
      .getAttribute("data-stroke-color");

    rerenderWith({ ...fixture, redrawingEdgeId: "E1" });

    expect(screen.getByTestId("polyline")).not.toHaveAttribute(
      "data-stroke-color",
      ordinaryColor,
    );
  });

  it("leaves every other saved cable in the ordinary colour while one is being redrawn", () => {
    renderCanvas({
      nodes: [
        node({ nodeId: "ODC-01", latitude: -6.2, longitude: 106.8 }),
        node({ nodeId: "ODP-01", latitude: -6.21, longitude: 106.81 }),
        node({ nodeId: "ONT-09", latitude: -6.22, longitude: 106.82 }),
      ],
      edges: [
        edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" }),
        edge({
          edgeId: "E2",
          source: "ODP-01",
          target: "ONT-09",
          waypoints: [{ lat: -6.215, lng: 106.815 }],
        }),
      ],
      redrawingEdgeId: "E2",
    });

    const polylines = screen.getAllByTestId("polyline");
    // Told apart by point count (2 vs 3), since the mock does not expose
    // which edge a polyline came from.
    const other = polylines.find(
      (el) => el.getAttribute("data-points") === "2",
    )!;
    const redrawing = polylines.find(
      (el) => el.getAttribute("data-points") === "3",
    )!;

    expect(other.getAttribute("data-stroke-color")).not.toBe(
      redrawing.getAttribute("data-stroke-color"),
    );
  });
});
