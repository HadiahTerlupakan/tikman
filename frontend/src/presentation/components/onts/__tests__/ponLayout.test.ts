import { describe, expect, it } from "vitest";
import type { PonHealth } from "@/domain/entities";
import { layoutPonTree } from "../ponLayout";

const health: PonHealth = {
  oltId: "olt-1",
  oltName: "Cariu",
  medianTrapPerOnt: 19,
  trapThreshold: 100,
  outageThreshold: 0.05,
  cards: [
    {
      slot: 8,
      ponCount: 1,
      pons: [
        {
          port: 12,
          ontCount: 41,
          trapPerOnt: 686,
          outageShare: 0.12,
          worst: [
            {
              ontId: "ont-1",
              label: "ONU-8:12",
              name: "MAD SURYA",
              trapCount: 1204,
              downMinutes: 340,
            },
          ],
        },
      ],
    },
  ],
};

describe("layoutPonTree", () => {
  it("places one node per level of the tree", () => {
    const { nodes } = layoutPonTree(health);

    expect(nodes.map((n) => n.kind)).toEqual(["olt", "card", "pon", "ont"]);
  });

  it("puts each level in its own column, left to right", () => {
    const { nodes } = layoutPonTree(health);
    const x = nodes.map((n) => n.x);

    expect(x[0]).toBeLessThan(x[1]);
    expect(x[1]).toBeLessThan(x[2]);
    expect(x[2]).toBeLessThan(x[3]);
  });

  it("connects every node to its parent", () => {
    const { edges } = layoutPonTree(health);

    // Three edges for four nodes: a tree, not a mesh.
    expect(edges).toHaveLength(3);
    expect(edges[0].path).toMatch(/^M /);
  });

  it("scores severity against the worst port drawn", () => {
    const { nodes } = layoutPonTree(health);
    const pon = nodes.find((n) => n.kind === "pon");

    // The only port drawn is by definition the worst one, so it anchors the
    // scale the colours read from.
    expect(pon?.severity).toBe(1);
  });

  it("returns a canvas big enough for every node", () => {
    const { nodes, width, height } = layoutPonTree(health);

    for (const node of nodes) {
      expect(node.x + node.width).toBeLessThanOrEqual(width);
      expect(node.y + node.height).toBeLessThanOrEqual(height);
    }
  });

  it("draws nothing for an OLT with no troubled port", () => {
    const { nodes, edges } = layoutPonTree({ ...health, cards: [] });

    expect(nodes).toHaveLength(0);
    expect(edges).toHaveLength(0);
  });
});
