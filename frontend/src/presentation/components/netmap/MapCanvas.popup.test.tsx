import type { ComponentProps, ReactNode } from "react";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { MapCanvas } from "./MapCanvas";

// NodePopup fetches an ODP's subscriber count for real; none of these tests
// are about that, so it is mocked to nothing rather than left to hit a
// QueryClient this file never sets up.
vi.mock("@/application/hooks", () => ({
  useOdpSubscribers: () => ({ data: undefined }),
}));

// Split out of MapCanvas.test.tsx the same way NetworkMapPage split deletion
// and redraw into their own files: the popup is a big enough surface (two
// content components, position resolution, a missing-endpoint fallback, and
// the must-not-interfere-with-tracing guard) to keep either file under the
// project's line limit. The mock below is a fuller copy of MapCanvas.test's
// own rather than a shared import, for the same reason those page-level
// files duplicate theirs: vi.mock factories are hoisted above this file's
// own imports.
vi.mock("@vis.gl/react-google-maps", () => ({
  APIProvider: ({ children }: { children: ReactNode }) => <>{children}</>,
  Map: ({ children }: { children: ReactNode }) => (
    <div data-testid="map">{children}</div>
  ),
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
  Polyline: ({ onClick }: { onClick?: () => void }) => (
    <div data-testid="polyline" onClick={onClick} />
  ),
  InfoWindow: ({
    position,
    onCloseClick,
    children,
  }: {
    position: { lat: number; lng: number };
    onCloseClick?: () => void;
    children: ReactNode;
  }) => (
    <div
      data-testid="info-window"
      data-lat={position.lat}
      data-lng={position.lng}
    >
      <button
        type="button"
        aria-label="tutup-info-window"
        onClick={onCloseClick}
      />
      {children}
    </div>
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

type Popup = ComponentProps<typeof MapCanvas>["popup"];

// MapCanvasProps.popup groups selection + close + the five action callbacks
// into one object (see MapCanvas.tsx); this builds one with every action
// defaulted to a no-op, so a test only has to name the piece it cares about.
function popup(overrides: Partial<Popup> = {}): Popup {
  return {
    onClose: noop,
    actions: {
      onEditNode: noop,
      onDeleteNode: noop,
      onEditEdge: noop,
      onRedrawEdge: noop,
      onDeleteEdge: noop,
    },
    ...overrides,
  };
}

const defaultProps: ComponentProps<typeof MapCanvas> = {
  nodes: [],
  edges: [],
  tracing: { draft: [] },
  placing: undefined,
  apiKey: "AIzaTEST",
  onDrop: noop,
  onNodeClick: noop,
  onEdgeClick: noop,
  popup: popup(),
};

describe("MapCanvas popups", () => {
  it("shows no popup when nothing is selected", () => {
    render(<MapCanvas {...defaultProps} />);

    expect(screen.queryByTestId("info-window")).not.toBeInTheDocument();
  });

  it("opens a node's popup anchored at its own coordinates", () => {
    const target = node({
      nodeId: "ODP-01",
      name: "ODP Satu",
      latitude: -6.21,
      longitude: 106.81,
    });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[target]}
        popup={popup({ selectedNode: target })}
      />,
    );

    const infoWindow = screen.getByTestId("info-window");
    expect(infoWindow).toHaveAttribute("data-lat", "-6.21");
    expect(infoWindow).toHaveAttribute("data-lng", "106.81");
    expect(within(infoWindow).getByText("ODP Satu")).toBeInTheDocument();
  });

  // Distinct coordinates on each end, with the anchor asserted, is what makes
  // this able to tell "anchors on source" from a reversed `target ?? source`
  // priority — both fixtures sharing the factory's default coordinates could
  // not, and neither could a test that only checked the names resolved.
  it("opens a cable's popup, resolving both ends from the node list and anchoring on the source", () => {
    const source = node({
      nodeId: "ODC-01",
      name: "ODC Satu",
      latitude: -6.2,
      longitude: 106.8,
    });
    const target = node({
      nodeId: "ODP-01",
      name: "ODP Satu",
      latitude: -6.3,
      longitude: 106.9,
    });
    const selected = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[source, target]}
        edges={[selected]}
        popup={popup({ selectedEdge: selected })}
      />,
    );

    expect(screen.getByText("ODC Satu → ODP Satu")).toBeInTheDocument();
    const infoWindow = screen.getByTestId("info-window");
    expect(infoWindow).toHaveAttribute("data-lat", "-6.2");
    expect(infoWindow).toHaveAttribute("data-lng", "106.8");
  });

  // Node deletion never cascades to edges (migration 53's own design). The
  // popup has to survive that, anchored at whichever end still exists rather
  // than crashing for lack of a position to anchor to.
  it("names a deleted endpoint honestly and anchors on the end that still exists", () => {
    const target = node({
      nodeId: "ODP-01",
      name: "ODP Satu",
      latitude: -6.22,
      longitude: 106.82,
    });
    const selected = edge({
      edgeId: "E1",
      source: "GHOST-404",
      target: "ODP-01",
    });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[target]}
        edges={[selected]}
        popup={popup({ selectedEdge: selected })}
      />,
    );

    expect(
      screen.getByText("Node sudah dihapus → ODP Satu"),
    ).toBeInTheDocument();
    const infoWindow = screen.getByTestId("info-window");
    expect(infoWindow).toHaveAttribute("data-lat", "-6.22");
    expect(infoWindow).toHaveAttribute("data-lng", "106.82");
  });

  // "No popup, no interference" — a popup left open from before tracing
  // started must not linger over the map while a cable is being traced.
  it("shows no popup while a cable is being traced, even if something is selected", () => {
    const target = node({ nodeId: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[target]}
        popup={popup({ selectedNode: target })}
        placing="cable"
      />,
    );

    expect(screen.queryByTestId("info-window")).not.toBeInTheDocument();
  });

  it("reports which cable was clicked when not tracing", async () => {
    const onEdgeClick = vi.fn();
    const source = node({ nodeId: "ODC-01" });
    const target = node({ nodeId: "ODP-01" });
    const selected = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[source, target]}
        edges={[selected]}
        onEdgeClick={onEdgeClick}
      />,
    );

    await userEvent.click(screen.getByTestId("polyline"));

    expect(onEdgeClick).toHaveBeenCalledWith("E1");
  });

  // A technician tracing a new cable along an existing one has to be able to
  // drop a corner on top of it; a click handler attached to the old cable's
  // line while tracing would swallow that tap instead of the map receiving
  // it, which is a regression a no-op guard alone would not catch.
  it("does not report a click on a cable's line while a cable is being traced", async () => {
    const onEdgeClick = vi.fn();
    const source = node({ nodeId: "ODC-01" });
    const target = node({ nodeId: "ODP-01" });
    const selected = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[source, target]}
        edges={[selected]}
        placing="cable"
        onEdgeClick={onEdgeClick}
      />,
    );

    await userEvent.click(screen.getByTestId("polyline"));

    expect(onEdgeClick).not.toHaveBeenCalled();
  });

  it("closes the popup through the InfoWindow's own close control", async () => {
    const onClose = vi.fn();
    const target = node({ nodeId: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[target]}
        popup={popup({ selectedNode: target, onClose })}
      />,
    );

    await userEvent.click(screen.getByLabelText("tutup-info-window"));

    expect(onClose).toHaveBeenCalled();
  });

  // The closest, most likely mistake once both popups exist side by side in
  // the same component — swapping which delete callback a popup's Hapus
  // reaches — would pass everything else and only show up here.
  it("wires the node popup's Hapus to onDeleteNode, not onDeleteEdge", async () => {
    const onDeleteNode = vi.fn();
    const onDeleteEdge = vi.fn();
    const target = node({ nodeId: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[target]}
        popup={popup({
          selectedNode: target,
          actions: {
            onEditNode: noop,
            onDeleteNode,
            onEditEdge: noop,
            onRedrawEdge: noop,
            onDeleteEdge,
          },
        })}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(onDeleteNode).toHaveBeenCalledWith("ODP-01");
    expect(onDeleteEdge).not.toHaveBeenCalled();
  });

  it("wires the cable popup's Hapus to onDeleteEdge, not onDeleteNode", async () => {
    const onDeleteNode = vi.fn();
    const onDeleteEdge = vi.fn();
    const source = node({ nodeId: "ODC-01" });
    const target = node({ nodeId: "ODP-01" });
    const selected = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[source, target]}
        edges={[selected]}
        popup={popup({
          selectedEdge: selected,
          actions: {
            onEditNode: noop,
            onDeleteNode,
            onEditEdge: noop,
            onRedrawEdge: noop,
            onDeleteEdge,
          },
        })}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Hapus" }));
    await userEvent.click(await screen.findByRole("button", { name: "Ya" }));

    expect(onDeleteEdge).toHaveBeenCalledWith("E1");
    expect(onDeleteNode).not.toHaveBeenCalled();
  });

  it("wires the cable popup's Gambar ulang to onRedrawEdge", async () => {
    const onRedrawEdge = vi.fn();
    const source = node({ nodeId: "ODC-01" });
    const target = node({ nodeId: "ODP-01" });
    const selected = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[source, target]}
        edges={[selected]}
        popup={popup({
          selectedEdge: selected,
          actions: {
            onEditNode: noop,
            onDeleteNode: noop,
            onEditEdge: noop,
            onRedrawEdge,
            onDeleteEdge: noop,
          },
        })}
      />,
    );

    await userEvent.click(screen.getByRole("button", { name: "Gambar ulang" }));

    expect(onRedrawEdge).toHaveBeenCalledWith(selected);
  });

  // NodePopup computes its slot usage from the `edges` MapCanvas already
  // has loaded to draw the map; this only passes if that prop actually
  // makes it all the way down through SelectedPopups/NodePopupWindow.
  it("shows the node's network position, proving the loaded edges reach its popup", () => {
    const odc = node({ nodeId: "ODC-01", type: "odc", capacity: 1 });
    const odp = node({ nodeId: "ODP-01", type: "odp" });
    const cable = edge({ edgeId: "E1", source: "ODC-01", target: "ODP-01" });
    render(
      <MapCanvas
        {...defaultProps}
        nodes={[odc, odp]}
        edges={[cable]}
        popup={popup({ selectedNode: odc })}
      />,
    );

    expect(screen.getByText("Kabel tergambar")).toBeInTheDocument();
    expect(screen.getByText("1 dari 1")).toBeInTheDocument();
  });
});
