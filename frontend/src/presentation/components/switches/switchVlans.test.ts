import { describe, expect, it } from "vitest";
import { matrixPorts, parseVlanList, vlanMembership } from "./switchVlans";
import { fixturePort, fixturePorts } from "./switchFixtures";

describe("parseVlanList", () => {
  it("expands ranges and ignores blanks", () => {
    expect([...parseVlanList("10,20-22, ")].sort((a, b) => a - b)).toEqual([
      10, 20, 21, 22,
    ]);
    expect(parseVlanList("").size).toBe(0);
  });
});

describe("vlanMembership", () => {
  const [trunk, access, hybrid] = fixturePorts;

  it("marks an access port untagged on its PVID only", () => {
    expect(vlanMembership(access, 30)).toBe("U");
    expect(vlanMembership(access, 10)).toBe("");
  });

  it("marks a trunk's allowed VLANs tagged, its PVID untagged when allowed", () => {
    expect(vlanMembership(trunk, 21)).toBe("T");
    expect(vlanMembership(trunk, 30)).toBe("");
    const native = fixturePort({
      name: "x",
      mode: "trunk",
      pvid: 10,
      allowedVlans: "10,20",
    });
    expect(vlanMembership(native, 10)).toBe("U");
    expect(vlanMembership(native, 20)).toBe("T");
  });

  it("reads a hybrid port like a trunk", () => {
    expect(vlanMembership(hybrid, 10)).toBe("U");
    expect(vlanMembership(hybrid, 30)).toBe("T");
  });

  it("leaves a port without a mode out", () => {
    expect(vlanMembership(fixturePort({ name: "y" }), 10)).toBe("");
  });
});

describe("matrixPorts", () => {
  it("keeps switched ports in natural order and drops the rest", () => {
    const ports = [
      fixturePort({ name: "10GE1/0/10", mode: "access", pvid: 1 }),
      ...fixturePorts,
    ];
    expect(matrixPorts(ports).map((p) => p.name)).toEqual([
      "10GE1/0/1",
      "10GE1/0/2",
      "10GE1/0/3",
      "10GE1/0/10",
    ]);
  });
});
