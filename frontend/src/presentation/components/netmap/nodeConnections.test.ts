import { describe, expect, it } from "vitest";
import type { MappingEdge, MappingNode, NodeType } from "@/domain/entities";
import { nodeConnections } from "./nodeConnections";

function node(id: string, type: NodeType): MappingNode {
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

describe("nodeConnections", () => {
  it("is empty in both directions for a node with no cables at all", () => {
    const odp = node("ODP-01", "odp");

    expect(nodeConnections(odp, [], byId([odp]))).toEqual({
      incoming: [],
      outgoing: [],
    });
  });

  it("names what feeds a node by its source node and fiber type", () => {
    const odc = node("ODC-01", "odc");
    const odp = node("ODP-01", "odp");
    const edges = [edge("ODC-01", "ODP-01", "distribution")];

    const result = nodeConnections(odp, edges, byId([odc, odp]));

    expect(result.incoming).toEqual([{ fiberType: "distribution", node: odc }]);
  });

  it("resolves no source node for a dangling incoming cable, without throwing", () => {
    const odp = node("ODP-01", "odp");
    const edges = [edge("GHOST-404", "ODP-01", "distribution")];

    const result = nodeConnections(odp, edges, byId([odp]));

    expect(result.incoming).toEqual([
      { fiberType: "distribution", node: undefined },
    ]);
  });

  it("counts what hangs off a node grouped by the kind of thing at the other end", () => {
    const odc = node("ODC-01", "odc");
    const odps = [
      node("ODP-01", "odp"),
      node("ODP-02", "odp"),
      node("ODP-03", "odp"),
    ];
    const odc2 = node("ODC-02", "odc");
    const edges = [
      ...odps.map((odp) => edge("ODC-01", odp.nodeId)),
      edge("ODC-01", "ODC-02", "odc_to_odc"),
    ];

    const result = nodeConnections(odc, edges, byId([odc, ...odps, odc2]));

    // Fixed kind order (server, odc, odp, ont), not edge-array order, so the
    // line reads the same regardless of the order cables were drawn in.
    expect(result.outgoing).toEqual([
      { type: "odc", count: 1 },
      { type: "odp", count: 3 },
    ]);
  });

  it("leaves a dangling outgoing cable out of the by-kind counts instead of guessing its kind", () => {
    const odc = node("ODC-01", "odc");
    const odp = node("ODP-01", "odp");
    const edges = [edge("ODC-01", "ODP-01"), edge("ODC-01", "GHOST-404")];

    const result = nodeConnections(odc, edges, byId([odc, odp]));

    expect(result.outgoing).toEqual([{ type: "odp", count: 1 }]);
  });

  it("counts a cable only on the end it actually touches", () => {
    const odc = node("ODC-01", "odc");
    const odp = node("ODP-01", "odp");
    const edges = [edge("ODC-01", "ODP-01")];

    expect(nodeConnections(odc, edges, byId([odc, odp])).incoming).toEqual([]);
    expect(nodeConnections(odp, edges, byId([odc, odp])).outgoing).toEqual([]);
  });
});
