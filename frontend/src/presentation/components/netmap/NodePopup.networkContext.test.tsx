import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MappingEdge, MappingNode, Ont } from "@/domain/entities";
import { NodePopup } from "./NodePopup";

// Split out of NodePopup.test.tsx the same way MapCanvas split its popup
// tests into their own file: this is its own surface (edges/nodesById wired
// through to slot usage, connections and the ODP subscriber fetch), not more
// cases of the identity/attributes/actions this component already covered,
// and keeping both in one file would run past the project's line limit.
// vi.mock factories are hoisted above this file's own imports, so the mock
// below is its own copy rather than a shared import.

// Real subscriber data is keyed by a node's real id (see DistributionRepository.
// toOdp), on purpose reusing the same string as a node id with no override
// data in the "not an ODP" test below, which depends on that overlap to prove
// the gate is the node's type, not an accident of which ids happen to have
// data.
const subscribersByOdpId: Record<string, Ont[]> = {
  "odp-real-id": [
    { id: "ont-1", serialNumber: "ZTEGC0000001" } as Ont,
    { id: "ont-2", serialNumber: "ZTEGC0000002" } as Ont,
    { id: "ont-3", serialNumber: "ZTEGC0000003" } as Ont,
  ],
};

vi.mock("@/application/hooks", () => ({
  useOdpSubscribers: (odpId?: string) => ({
    data: odpId ? subscribersByOdpId[odpId] : undefined,
  }),
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
  source: string,
  target: string,
  fiberType: MappingEdge["fiberType"] = "distribution",
): MappingEdge {
  return {
    edgeId: `${source}--${target}`,
    source,
    target,
    fiberType,
    distance: 0,
    waypoints: [],
    notes: "",
  };
}

function byId(nodes: MappingNode[]): Map<string, MappingNode> {
  return new Map(nodes.map((n) => [n.nodeId, n]));
}

describe("NodePopup network context", () => {
  it("shows slot usage computed from the loaded edges and nodes", () => {
    const odc = node({ nodeId: "ODC-01", type: "odc", capacity: 8 });
    const odp = node({ nodeId: "ODP-01", type: "odp" });
    render(
      <NodePopup
        node={odc}
        edges={[edge("ODC-01", "ODP-01")]}
        nodesById={byId([odc, odp])}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Kabel tergambar")).toBeInTheDocument();
    expect(screen.getByText("1 dari 8")).toBeInTheDocument();
  });

  it("shows what feeds this node and what hangs off it", () => {
    const server = node({ nodeId: "SRV-01", type: "server" });
    const odc = node({ nodeId: "ODC-01", type: "odc" });
    const odp = node({ nodeId: "ODP-01", type: "odp" });
    render(
      <NodePopup
        node={odc}
        edges={[edge("SRV-01", "ODC-01", "feeder"), edge("ODC-01", "ODP-01")]}
        nodesById={byId([server, odc, odp])}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(
      screen.getByText("Dari: SRV-01 (Feeder) · Ke: 1 ODP"),
    ).toBeInTheDocument();
  });

  it("shows an ODP's real subscriber count, fetched by the node's own id", () => {
    const odp = node({
      nodeId: "ODP-01",
      id: "odp-real-id",
      type: "odp",
    });
    render(
      <NodePopup
        node={odp}
        edges={[]}
        nodesById={new Map()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.getByText("Pelanggan terdaftar")).toBeInTheDocument();
    expect(screen.getByText("3 ONT")).toBeInTheDocument();
  });

  // Reuses "odp-real-id" as this node's nodeId — a key the mock actually has
  // subscriber data under — so this only passes if the popup withholds the
  // fetch because of the node's type, not because that id has no data.
  it("does not fetch or show a subscriber count for a node that is not an ODP", () => {
    const odc = node({ nodeId: "odp-real-id", type: "odc" });
    render(
      <NodePopup
        node={odc}
        edges={[]}
        nodesById={new Map()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByText(/Pelanggan/)).not.toBeInTheDocument();
  });

  it("shows nothing extra for a freshly placed node with no cables, no capacity, and no subscribers", () => {
    render(
      <NodePopup
        node={node({ nodeId: "SRV-01", type: "server" })}
        edges={[]}
        nodesById={new Map()}
        onEdit={vi.fn()}
        onDelete={vi.fn()}
        onClose={vi.fn()}
      />,
    );

    expect(screen.queryByText("Posisi jaringan")).not.toBeInTheDocument();
  });
});
