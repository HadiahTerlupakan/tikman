import { describe, expect, it } from "vitest";
import { cleanLabel, isIdlePort, portGroups, vlanLabel } from "./vlanMatrix";
import { fixturePort } from "./switchFixtures";

describe("cleanLabel", () => {
  it("drops the quotes and stars a CE description is often wrapped in", () => {
    expect(cleanLabel('"NNI TO FIBERNET"')).toBe("NNI TO FIBERNET");
    expect(cleanLabel("** NNI To DINETKAN **")).toBe("NNI To DINETKAN");
    expect(cleanLabel("  JKT-IX ")).toBe("JKT-IX");
    expect(cleanLabel("")).toBe("");
  });
});

describe("vlanLabel", () => {
  it("hides the name a CE makes up for an unnamed VLAN", () => {
    expect(vlanLabel({ switchId: "s", vid: 1, name: "VLAN 0001" })).toBe("");
    expect(vlanLabel({ switchId: "s", vid: 10, name: "MGMT-VLAN" })).toBe(
      "MGMT-VLAN",
    );
    expect(vlanLabel({ switchId: "s", vid: 302, name: '"qinq dacen 3' })).toBe(
      "qinq dacen 3",
    );
  });
});

describe("isIdlePort", () => {
  it("counts a down, undescribed port left on the default VLAN as idle", () => {
    const idle = fixturePort({
      name: "10GE1/0/30",
      mode: "access",
      pvid: 1,
      operStatus: "down",
    });
    expect(isIdlePort(idle)).toBe(true);
  });

  it("keeps a port that is up, described, or moved off VLAN 1", () => {
    const base = { mode: "access", pvid: 1, operStatus: "down" } as const;
    expect(
      isIdlePort(fixturePort({ name: "a", ...base, operStatus: "up" })),
    ).toBe(false);
    expect(isIdlePort(fixturePort({ name: "b", ...base, alias: "AIX" }))).toBe(
      false,
    );
    expect(isIdlePort(fixturePort({ name: "c", ...base, pvid: 2506 }))).toBe(
      false,
    );
    expect(
      isIdlePort(
        fixturePort({ name: "d", ...base, mode: "trunk", allowedVlans: "10" }),
      ),
    ).toBe(false);
  });
});

describe("portGroups", () => {
  it("groups ports by slot prefix and keeps the short number", () => {
    const groups = portGroups([
      fixturePort({ name: "10GE1/0/1" }),
      fixturePort({ name: "10GE1/0/2" }),
      fixturePort({ name: "40GE1/0/1" }),
      fixturePort({ name: "Eth-Trunk1" }),
    ]);
    expect(groups.map((g) => g.prefix)).toEqual([
      "10GE1/0/",
      "40GE1/0/",
      "Eth-Trunk",
    ]);
    expect(groups[0].ports.map((p) => p.short)).toEqual(["1", "2"]);
    expect(groups[2].ports[0].short).toBe("1");
    expect(portGroups([fixturePort({ name: "NULL" })])[0]).toMatchObject({
      prefix: "",
      ports: [{ short: "NULL" }],
    });
  });
});
