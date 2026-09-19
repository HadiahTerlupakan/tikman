import { describe, expect, it } from "vitest";
import type { MappingEdge, MappingNode, NodeType } from "@/domain/entities";
import { slotUsage } from "./slotUsage";

function node(id: string, type: NodeType, capacity = 0): MappingNode {
  return {
    nodeId: id,
    type,
    name: id,
    latitude: -6.2,
    longitude: 106.8,
    capacity,
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

describe("slotUsage", () => {
  it("has nothing to count for a node type that is never a source in any capacity rule", () => {
    const server = node("SRV-01", "server", 8);
    const ont = node("ONT-01", "ont", 8);

    expect(slotUsage(server, [], byId([server]))).toBeUndefined();
    expect(slotUsage(ont, [], byId([ont]))).toBeUndefined();
  });

  // Mirrors TestACapacityOfZeroNeverRefuses in mapping_edges_test.go: nobody
  // has counted this box's ports, so it reads as unlimited regardless of how
  // many cables are actually drawn off it.
  it("reads as unlimited when capacity is zero, no matter how many cables are drawn", () => {
    const odc = node("ODC-01", "odc", 0);
    const odps = Array.from({ length: 20 }, (_, i) => node(`ODP-${i}`, "odp"));
    const edges = odps.map((odp) => edge("ODC-01", odp.nodeId));

    const result = slotUsage(odc, edges, byId([odc, ...odps]));

    expect(result).toEqual({ capacity: 0, unlimited: true, usages: [] });
  });

  it("counts an ODC's ordinary ODP drops", () => {
    const odc = node("ODC-01", "odc", 8);
    const odp1 = node("ODP-01", "odp");
    const odp2 = node("ODP-02", "odp");
    const edges = [edge("ODC-01", "ODP-01"), edge("ODC-01", "ODP-02")];

    const result = slotUsage(odc, edges, byId([odc, odp1, odp2]));

    expect(result?.usages).toContainEqual({
      targetType: "odp",
      cascade: false,
      used: 2,
    });
  });

  // The trap: folding a cascade into the ordinary count would make an ODC
  // with 8 ODPs and 2 ODC cascades read as 10/8 instead of two separate
  // figures. A mutant that merges the two pools would report used: 10 here.
  it("counts an ODC's own-kind cascades apart from its ordinary ODP drops", () => {
    const odc = node("ODC-01", "odc", 8);
    const odps = Array.from({ length: 8 }, (_, i) => node(`ODP-0${i}`, "odp"));
    const odc2 = node("ODC-02", "odc");
    const odc3 = node("ODC-03", "odc");
    const edges = [
      ...odps.map((odp) => edge("ODC-01", odp.nodeId, "distribution")),
      edge("ODC-01", "ODC-02", "odc_to_odc"),
      edge("ODC-01", "ODC-03", "odc_to_odc"),
    ];

    const result = slotUsage(odc, edges, byId([odc, ...odps, odc2, odc3]));

    expect(result?.usages).toHaveLength(2);
    expect(result?.usages).toContainEqual({
      targetType: "odp",
      cascade: false,
      used: 8,
    });
    expect(result?.usages).toContainEqual({
      targetType: "odc",
      cascade: true,
      used: 2,
    });
  });

  // Mirrors TestACascadeIsCountedApartFromOrdinaryDrops in mapping_edges_test.go.
  it("counts an ODP's ordinary ONT drops apart from its ODP-to-ODP cascades", () => {
    const odp = node("ODP-01", "odp", 1);
    const ont = node("ONT-01", "ont");
    const odp2 = node("ODP-02", "odp");
    const edges = [
      edge("ODP-01", "ONT-01", "drop"),
      edge("ODP-01", "ODP-02", "odp_to_odp"),
    ];

    const result = slotUsage(odp, edges, byId([odp, ont, odp2]));

    expect(result?.usages).toContainEqual({
      targetType: "ont",
      cascade: false,
      used: 1,
    });
    expect(result?.usages).toContainEqual({
      targetType: "odp",
      cascade: true,
      used: 1,
    });
  });

  // slotKind() in the backend matches the cascade fiber type exactly; the
  // "_ratio" variant is a different FiberType constant and is not counted by
  // any rule at all, same as the backend. A `.startsWith`/`.includes` mistake
  // here would still pass a test that only checked the exact string.
  it("does not count an ODC-to-ODC cable drawn with the splitter-ratio fiber type as a cascade", () => {
    const odc = node("ODC-01", "odc", 8);
    const odc2 = node("ODC-02", "odc");
    const edges = [edge("ODC-01", "ODC-02", "odc_to_odc_ratio")];

    const result = slotUsage(odc, edges, byId([odc, odc2]));

    expect(result?.usages).toContainEqual({
      targetType: "odc",
      cascade: true,
      used: 0,
    });
  });

  it("does not count a cable whose target node cannot be resolved", () => {
    const odc = node("ODC-01", "odc", 8);
    const edges = [edge("ODC-01", "GHOST-404")];

    const result = slotUsage(odc, edges, byId([odc]));

    expect(result?.usages).toHaveLength(2);
    expect(result?.usages).toContainEqual({
      targetType: "odp",
      cascade: false,
      used: 0,
    });
  });

  it("does not count a cable where this node is the target, not the source", () => {
    const odp = node("ODP-01", "odp", 8);
    const odc = node("ODC-01", "odc");
    // ODP-01 is the target of this cable, never its source.
    const edges = [edge("ODC-01", "ODP-01")];

    const result = slotUsage(odp, edges, byId([odp, odc]));

    expect(result?.usages).toContainEqual({
      targetType: "ont",
      cascade: false,
      used: 0,
    });
  });
});
