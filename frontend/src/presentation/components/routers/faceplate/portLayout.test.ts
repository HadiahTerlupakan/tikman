import { describe, expect, it } from "vitest";
import type { RouterInterface } from "../../../../domain/entities/MikrotikRouter";
import { layoutFaceplate, physicalPortNames } from "./portLayout";

const iface = (name: string, type: string, parentName: string | null = null) =>
  ({ name, type, parentName }) as RouterInterface;

describe("layoutFaceplate", () => {
  it("draws every jack of a known board even before discovery reports them", () => {
    const layout = layoutFaceplate("CCR2116-12G-4S+", []);
    const by = Object.fromEntries(layout.jacks.map((j) => [j.name, j]));

    expect(layout.templated).toBe(true);
    expect(layout.extras).toBe(true);
    expect(layout.jacks).toHaveLength(17);
    expect(by["ether1"]).toMatchObject({ mgmt: true, row: "bottom" });
    expect(by["ether2"].row).toBe("top");
    expect(by["ether3"].row).toBe("bottom");
    expect(by["ether2"].x).toBe(by["ether3"].x);
    expect(by["sfp-sfpplus1"].kind).toBe("sfp");
  });

  it("groups an unknown board by name as rj, sfp, qsfp", () => {
    const layout = layoutFaceplate("CCR2216-1G-12XS-2XQ", [
      "ether1",
      "sfp28-1",
      "sfp28-2",
      "qsfp28-1-1",
    ]);

    expect(layout.templated).toBe(false);
    expect(layout.extras).toBe(false);
    expect(layout.jacks.map((j) => j.kind)).toEqual([
      "rj",
      "sfp",
      "sfp",
      "qsfp",
    ]);
    expect(layout.jacks[0].mgmt).toBe(true);
    expect(layout.jacks[3].name).toBe("qsfp28-1-1");
  });

  it("still draws a port renamed away from its template name", () => {
    const layout = layoutFaceplate("CCR2116-12G-4S+", [
      "ether1",
      "sfp1-transit",
      "sfp-sfpplus2",
    ]);
    const renamed = layout.jacks.find((j) => j.name === "sfp1-transit");

    expect(layout.jacks).toHaveLength(18);
    expect(renamed?.kind).toBe("sfp");
    expect(renamed?.x).toBeGreaterThan(
      Math.max(...layout.jacks.filter((j) => j !== renamed).map((j) => j.x)),
    );
  });

  it("keeps a port the operator extended in its own slot", () => {
    const names = [
      "ether1-x86",
      "ether12-wifi-bascamp",
      "sfp-sfpplus1-AGREGATOR",
    ];
    const layout = layoutFaceplate("CCR2116-12G-4S+", names);
    const plain = layoutFaceplate("CCR2116-12G-4S+", []);
    const at = (l: typeof layout, slot: string) =>
      l.jacks.find((j) => j.slot === slot);

    expect(layout.jacks).toHaveLength(plain.jacks.length);
    expect(layout.width).toBe(plain.width);
    expect(at(layout, "ether1")?.name).toBe("ether1-x86");
    expect(at(layout, "ether12")?.name).toBe("ether12-wifi-bascamp");
    expect(at(layout, "sfp-sfpplus1")?.name).toBe("sfp-sfpplus1-AGREGATOR");
    expect(at(layout, "ether12")?.x).toBe(at(plain, "ether12")?.x);
  });

  it("gives the slot to the exact name when both it and an extension exist", () => {
    const layout = layoutFaceplate("CCR2116-12G-4S+", ["ether1", "ether1-x86"]);
    expect(layout.jacks.find((j) => j.slot === "ether1")?.name).toBe("ether1");
    expect(layout.jacks.some((j) => j.name === "ether1-x86")).toBe(true);
  });

  it("does not seat ether12-something in the ether1 slot", () => {
    const layout = layoutFaceplate("CCR2116-12G-4S+", ["ether12-uplink"]);
    expect(layout.jacks.find((j) => j.slot === "ether1")?.name).toBe("ether1");
  });

  it("draws a port whose name fits no group of an unknown board", () => {
    const layout = layoutFaceplate(null, ["ether1", "ether2", "uplink-a"]);
    expect(layout.jacks.map((j) => j.name)).toEqual([
      "ether1",
      "ether2",
      "uplink-a",
    ]);
  });

  it("treats a null board like an unknown one", () => {
    expect(layoutFaceplate(null, ["ether1"]).templated).toBe(false);
  });

  it("has a width but no jacks for an empty unknown board", () => {
    const layout = layoutFaceplate(null, []);
    expect(layout.jacks).toEqual([]);
    expect(layout.width).toBeGreaterThan(0);
  });

  it("widens an automatic layout more than a templated one", () => {
    const auto = layoutFaceplate(null, ["ether1"]);
    expect(auto.width).toBe(40 + 46 + 44 + 330);
    const tpl = layoutFaceplate("CCR2004-16G-2S+", []);
    expect(tpl.jacks).toHaveLength(18);
    expect(tpl.width).toBe(166 + 7 * 58 + 46 + 44 + 58 + 44 + 230);
  });
});

describe("physicalPortNames", () => {
  it("keeps physical ports in natural order and drops virtual ones", () => {
    const names = physicalPortNames([
      iface("ether10", "ether"),
      iface("sfp-sfpplus1", "ether"),
      iface("vlan201", "vlan"),
      iface("vlan202", "ether", "sfp-sfpplus1"),
      iface("bridge1", "bridge"),
      iface("wg0", "wg"),
      iface("lte1", "lte"),
      iface("ether1", "ether"),
      iface("ether2", "ether"),
    ]);
    expect(names).toEqual(["ether1", "ether2", "ether10", "sfp-sfpplus1"]);
  });
});
