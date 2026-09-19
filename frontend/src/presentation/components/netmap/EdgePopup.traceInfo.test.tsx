import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { EdgePopup } from "./EdgePopup";

// Split out of EdgePopup.test.tsx the same way MapCanvas split its popup
// tests into their own file: how a cable was traced (item 4 of this
// feature) is its own surface, not more cases of the identity/details/
// actions EdgePopup.test.tsx already covered, and keeping both in one file
// would run past the project's line limit.

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

function edge(overrides: Partial<MappingEdge> = {}): MappingEdge {
  return {
    edgeId: "ODC-01--ODP-01",
    source: "ODC-01",
    target: "ODP-01",
    fiberType: "distribution",
    distance: 120,
    waypoints: [],
    notes: "",
    ...overrides,
  };
}

const sourceNode = node({ nodeId: "ODC-01", name: "ODP Masjid" });
const targetNode = node({ nodeId: "ODP-01", name: "Rumah Pak Budi" });

describe("EdgePopup trace info", () => {
  // sourceNode/targetNode share the node() factory's default coordinates,
  // so the straight line between them is exactly zero — an independently
  // obvious expected value, not one derived from the code under test.
  it("shows the traced length, the straight-line comparison, and the bend count", () => {
    render(
      <EdgePopup
        edge={edge({
          distance: 412,
          waypoints: [{ lat: -6.2, lng: 106.8 }],
        })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText("412 m (garis lurus 0 m) · 1 titik belok"),
    ).toBeInTheDocument();
  });

  it("counts zero bends for a cable drawn with no corners", () => {
    render(
      <EdgePopup
        edge={edge({ distance: 50, waypoints: [] })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText("50 m (garis lurus 0 m) · 0 titik belok"),
    ).toBeInTheDocument();
  });

  // Reuses the same reference fact cableMath's own test establishes
  // independently (a hundredth of a degree of latitude is about a
  // kilometre), rather than computing the expected value by calling the
  // code under test on itself.
  it("computes the straight line from the endpoints' real coordinates, not a fixed number", () => {
    const far = node({
      nodeId: "ODP-02",
      name: "Jauh",
      latitude: -6.21,
      longitude: 106.8,
    });
    render(
      <EdgePopup
        edge={edge({ distance: 5000, waypoints: [] })}
        sourceNode={sourceNode}
        targetNode={far}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText(/garis lurus 1,\d\d km/)).toBeInTheDocument();
  });

  // Node deletion never cascades to edges, so an endpoint can be gone; the
  // straight line has no coordinates to work from, but the traced length
  // and bend count are still known from the edge itself.
  it("omits the straight-line comparison when an endpoint is gone, but still shows the rest", () => {
    render(
      <EdgePopup
        edge={edge({
          distance: 200,
          waypoints: [{ lat: -6.2, lng: 106.8 }],
        })}
        sourceNode={undefined}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("200 m · 1 titik belok")).toBeInTheDocument();
    expect(screen.queryByText(/garis lurus/)).not.toBeInTheDocument();
  });

  it("separates the trace info from the cable's own details with a top border", () => {
    render(
      <EdgePopup
        edge={edge({ waypoints: [] })}
        sourceNode={sourceNode}
        targetNode={targetNode}
        onEdit={vi.fn()}
        onRedraw={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    const traceBlock = screen.getByText(/titik belok/).closest(".ant-space");
    expect(traceBlock).toHaveStyle({ borderTop: "1px solid #27272a" });
  });
});
