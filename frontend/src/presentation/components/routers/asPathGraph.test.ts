import { describe, expect, it } from "vitest";
import { buildPathGraph, ROUTER_NODE, type PathSource } from "./asPathGraph";

function path(over: Partial<PathSource>): PathSource {
  return {
    prefix: "198.51.100.0/24",
    peer: "peer-ix",
    asPath: "64500,64496",
    active: true,
    bytes: 100,
    ...over,
  };
}

describe("buildPathGraph", () => {
  it("lays one path out as a chain from the router, one AS per column", () => {
    const g = buildPathGraph([path({})]);

    expect(g.columns).toBe(3);
    expect(g.nodes.map((n) => [n.id, n.column, n.asn])).toEqual([
      [ROUTER_NODE, 0, null],
      ["1:64500", 1, 64500],
      ["2:64496", 2, 64496],
    ]);
    expect(g.edges.map((e) => [e.from, e.to, e.bytes, e.active])).toEqual([
      [ROUTER_NODE, "1:64500", 100, true],
      ["1:64500", "2:64496", 100, true],
    ]);
  });

  it("merges the ASes two prefixes share and sums their traffic", () => {
    const g = buildPathGraph([
      path({ prefix: "198.51.100.0/24", bytes: 100 }),
      path({ prefix: "203.0.113.0/24", asPath: "64500,64497", bytes: 50 }),
    ]);

    const peer = g.nodes.find((n) => n.id === "1:64500");
    expect(peer?.bytes).toBe(150);
    expect(peer?.prefixes).toEqual(["198.51.100.0/24", "203.0.113.0/24"]);
    expect(g.edges.find((e) => e.to === "1:64500")?.bytes).toBe(150);
    expect(g.nodes.filter((n) => n.column === 2).map((n) => n.asn)).toEqual([
      64496, 64497,
    ]);
  });

  it("collapses prepends so a padded path keeps its real hops", () => {
    const g = buildPathGraph([path({ asPath: "64501,64501,64501,64496" })]);

    expect(g.nodes.map((n) => n.id)).toEqual([
      ROUTER_NODE,
      "1:64501",
      "2:64496",
    ]);
    expect(g.nodes[1].prepends).toBe(2);
  });

  it("keeps alternatives dashed and a node active when any active path uses it", () => {
    const g = buildPathGraph([
      path({ asPath: "64500,64496", active: true }),
      path({
        peer: "peer-transit",
        asPath: "64501,64502,64496",
        active: false,
      }),
    ]);

    expect(g.edges.find((e) => e.to === "1:64501")?.active).toBe(false);
    expect(g.nodes.find((n) => n.id === "2:64502")?.active).toBe(false);
    expect(g.nodes.find((n) => n.id === "1:64500")?.active).toBe(true);
    expect(g.nodes.find((n) => n.id === "3:64496")?.origin).toBe(true);
  });

  it("marks the last AS of a path as the origin and records the peers at the first", () => {
    const g = buildPathGraph([
      path({ peer: "peer-ix-rs1" }),
      path({ peer: "peer-ix-rs2", prefix: "203.0.113.0/24" }),
    ]);

    expect(g.nodes.find((n) => n.id === "1:64500")?.peers).toEqual([
      "peer-ix-rs1",
      "peer-ix-rs2",
    ]);
    expect(g.nodes.find((n) => n.id === "1:64500")?.origin).toBe(false);
    expect(g.nodes.find((n) => n.id === "2:64496")?.origin).toBe(true);
  });

  it("orders the nodes of a column busiest first", () => {
    const g = buildPathGraph([
      path({ asPath: "64500,64496", bytes: 10 }),
      path({ prefix: "203.0.113.0/24", asPath: "64501,64497", bytes: 90 }),
    ]);

    expect(g.nodes.filter((n) => n.column === 1).map((n) => n.asn)).toEqual([
      64501, 64500,
    ]);
  });

  it("skips a path with no AS hops, such as a locally originated route", () => {
    const g = buildPathGraph([path({ asPath: "" })]);

    expect(g.nodes).toEqual([]);
    expect(g.edges).toEqual([]);
    expect(g.columns).toBe(0);
  });

  it("reads an AS set and a space-separated path too", () => {
    const g = buildPathGraph([path({ asPath: "64500 {64496,64497}" })]);

    expect(g.nodes.map((n) => n.asn)).toEqual([null, 64500, 64496]);
  });
});
