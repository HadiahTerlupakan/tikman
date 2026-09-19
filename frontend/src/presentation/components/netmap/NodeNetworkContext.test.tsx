import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import type { MappingEdge, MappingNode, NodeType } from "@/domain/entities";
import { NodeNetworkContext } from "./NodeNetworkContext";

function node(
  id: string,
  type: NodeType,
  overrides: Partial<MappingNode> = {},
): MappingNode {
  return {
    nodeId: id,
    type,
    name: id,
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

describe("NodeNetworkContext", () => {
  it("renders nothing for an isolated node with no capacity rule and no cables", () => {
    const server = node("SRV-01", "server");
    const { container } = render(
      <NodeNetworkContext
        node={server}
        edges={[]}
        nodesById={byId([server])}
      />,
    );

    expect(container).toBeEmptyDOMElement();
  });

  it("shows how full a box is", () => {
    const odc = node("ODC-01", "odc", { capacity: 8 });
    const odp = node("ODP-01", "odp");
    const edges = [edge("ODC-01", "ODP-01")];

    render(
      <NodeNetworkContext
        node={odc}
        edges={edges}
        nodesById={byId([odc, odp])}
      />,
    );

    expect(screen.getByText("1 dari 8 terpakai")).toBeInTheDocument();
  });

  it("shows unlimited instead of a count when capacity is zero", () => {
    const odc = node("ODC-01", "odc", { capacity: 0 });
    render(
      <NodeNetworkContext node={odc} edges={[]} nodesById={byId([odc])} />,
    );

    expect(screen.getByText("tanpa batas")).toBeInTheDocument();
  });

  // The trap this whole feature exists to get right: the two figures must
  // stay visually separate, never combined into one "10 of 8".
  it("shows a cascade's usage as its own figure, separate from the ordinary count", () => {
    const odc = node("ODC-01", "odc", { capacity: 8 });
    const odps = Array.from({ length: 8 }, (_, i) => node(`ODP-0${i}`, "odp"));
    const odc2 = node("ODC-02", "odc");
    const edges = [
      ...odps.map((odp) => edge("ODC-01", odp.nodeId)),
      edge("ODC-01", "ODC-02", "odc_to_odc"),
    ];

    render(
      <NodeNetworkContext
        node={odc}
        edges={edges}
        nodesById={byId([odc, ...odps, odc2])}
      />,
    );

    expect(screen.getByText("8 dari 8 terpakai")).toBeInTheDocument();
    expect(
      screen.getByText("1 dari 8 untuk kaskade ke ODC"),
    ).toBeInTheDocument();
    expect(screen.queryByText(/10/)).not.toBeInTheDocument();
  });

  it("does not mention a cascade at all when none has been drawn", () => {
    const odc = node("ODC-01", "odc", { capacity: 8 });
    render(
      <NodeNetworkContext node={odc} edges={[]} nodesById={byId([odc])} />,
    );

    expect(screen.queryByText(/kaskade/)).not.toBeInTheDocument();
  });

  it("names what feeds a node and what hangs off it, counted by kind", () => {
    const odc = node("ODC-01", "odc", { name: "ODC Satu" });
    const server = node("SRV-01", "server");
    const odp1 = node("ODP-01", "odp");
    const odp2 = node("ODP-02", "odp");
    const ont = node("ONT-01", "ont");
    const edges = [
      edge("SRV-01", "ODC-01", "feeder"),
      edge("ODC-01", "ODP-01"),
      edge("ODC-01", "ODP-02"),
    ];

    render(
      <NodeNetworkContext
        node={odc}
        edges={edges}
        nodesById={byId([odc, server, odp1, odp2, ont])}
      />,
    );

    expect(
      screen.getByText("Dari: SRV-01 (Feeder) · Ke: 2 ODP"),
    ).toBeInTheDocument();
  });

  it("omits the Dari clause when nothing feeds a node, and the Ke clause when nothing hangs off it", () => {
    const odc = node("ODC-01", "odc");
    const odp = node("ODP-01", "odp");
    const edges = [edge("ODC-01", "ODP-01")];

    render(
      <NodeNetworkContext
        node={odp}
        edges={edges}
        nodesById={byId([odc, odp])}
      />,
    );

    expect(screen.getByText(/^Dari: ODC-01/)).toBeInTheDocument();
    expect(screen.queryByText(/Ke:/)).not.toBeInTheDocument();
  });

  it("names a deleted source node honestly in the Dari clause instead of crashing", () => {
    const odp = node("ODP-01", "odp");
    const edges = [edge("GHOST-404", "ODP-01", "drop")];

    render(
      <NodeNetworkContext node={odp} edges={edges} nodesById={byId([odp])} />,
    );

    expect(
      screen.getByText("Dari: Node sudah dihapus (Drop)"),
    ).toBeInTheDocument();
  });

  it("shows an ODP's real subscriber count when given one", () => {
    const odp = node("ODP-01", "odp");
    render(
      <NodeNetworkContext
        node={odp}
        edges={[]}
        nodesById={byId([odp])}
        subscriberCount={4}
      />,
    );

    expect(screen.getByText("Pelanggan: 4 ONT")).toBeInTheDocument();
  });

  it("shows zero subscribers as a real figure, not as nothing to show", () => {
    const odp = node("ODP-01", "odp");
    render(
      <NodeNetworkContext
        node={odp}
        edges={[]}
        nodesById={byId([odp])}
        subscriberCount={0}
      />,
    );

    expect(screen.getByText("Pelanggan: 0 ONT")).toBeInTheDocument();
  });

  it("says nothing about subscribers when no count was given", () => {
    const odp = node("ODP-01", "odp");
    render(
      <NodeNetworkContext node={odp} edges={[]} nodesById={byId([odp])} />,
    );

    expect(screen.queryByText(/Pelanggan/)).not.toBeInTheDocument();
  });

  it("carries its own section apart from the node's attributes, with a top border", () => {
    const odc = node("ODC-01", "odc", { capacity: 0 });
    render(
      <NodeNetworkContext node={odc} edges={[]} nodesById={byId([odc])} />,
    );

    const section = screen.getByText("tanpa batas").closest(".ant-space");
    expect(section).toHaveStyle({ borderTop: "1px solid #27272a" });
  });
});
