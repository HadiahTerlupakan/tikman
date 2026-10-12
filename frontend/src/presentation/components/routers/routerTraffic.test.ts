import { describe, expect, it } from "vitest";
import type {
  RouterBgpSession,
  RouterInterface,
  RouterTrafficPoint,
} from "@/domain/entities";
import {
  bgpSummary,
  directionLabels,
  filterInterfaces,
  formatBps,
  formatUptime,
  groupInterfacesByParent,
  groupSessionsByInterface,
  interfaceLabel,
  interfaceSeries,
  interfaceStatus,
  isUpstream,
  lastBgpRead,
  memoryUsedPercent,
  stackSeries,
} from "./routerTraffic";

const iface = (over: Partial<RouterInterface>): RouterInterface => ({
  id: "if",
  routerId: "r",
  name: "ether1",
  comment: "",
  type: "ether",
  vlanId: null,
  parentName: null,
  ifIndex: null,
  speedMbps: null,
  disabled: false,
  operStatus: "up",
  addresses: "",
  lastInBps: null,
  lastOutBps: null,
  lastPolledAt: null,
  lastTrafficAt: null,
  silentSince: null,
  ...over,
});

const session = (over: Partial<RouterBgpSession>): RouterBgpSession => ({
  id: "s",
  routerId: "r",
  name: "peer",
  remoteAddress: "192.0.2.1",
  remoteAs: 64500,
  localAddress: "192.0.2.0",
  state: "established",
  uptimeSeconds: 0,
  prefixCount: 0,
  lastStartedAt: null,
  lastStoppedAt: null,
  interfaceId: null,
  updatedAt: "2026-10-12T08:00:00Z",
  ...over,
});

describe("lastBgpRead", () => {
  it("is the newest time any session was written", () => {
    expect(
      lastBgpRead([
        session({ updatedAt: "2026-10-12T08:00:00Z" }),
        session({ updatedAt: "2026-10-12T08:05:00Z" }),
        session({ updatedAt: "2026-10-12T07:55:00Z" }),
      ]),
    ).toBe("2026-10-12T08:05:00Z");
  });

  it("is null for a router never read", () => {
    expect(lastBgpRead([])).toBeNull();
  });
});

describe("bgpSummary", () => {
  it("tallies sessions while REST answers", () => {
    expect(
      bgpSummary({ apiStatus: "ok", bgpEstablished: 3, bgpTotal: 5 }),
    ).toBe("3/5 Established");
  });

  it("claims nothing once REST stops answering, the tally being stale", () => {
    for (const apiStatus of [
      "error",
      "certificate_changed",
      "unknown",
    ] as const) {
      expect(bgpSummary({ apiStatus, bgpEstablished: 3, bgpTotal: 5 })).toBe(
        "—",
      );
    }
  });
});

describe("interfaceLabel", () => {
  it("prefers the comment, which is what operators named the link", () => {
    expect(interfaceLabel(iface({ name: "vlan-ix", comment: "CDN IX" }))).toBe(
      "CDN IX",
    );
    expect(interfaceLabel(iface({ name: "vlan-ix" }))).toBe("vlan-ix");
  });
});

describe("isUpstream and directionLabels", () => {
  it("calls an interface carrying a BGP session an upstream", () => {
    const ix = iface({ id: "ix" });
    const pop = iface({ id: "pop" });
    const sessions = [session({ interfaceId: "ix" })];
    expect(isUpstream(ix, sessions)).toBe(true);
    expect(isUpstream(pop, sessions)).toBe(false);
  });

  it("reads an upstream's rx as download and a POP's tx as towards subscribers", () => {
    expect(directionLabels(true)).toEqual({ in: "Download", out: "Upload" });
    expect(directionLabels(false)).toEqual({
      in: "Dari pelanggan",
      out: "Ke pelanggan",
    });
  });
});

describe("groupSessionsByInterface", () => {
  it("puts every route-server session under its shared VLAN and unmatched ones last", () => {
    const vlan = iface({ id: "rs", comment: "IX route servers" });
    const groups = groupSessionsByInterface(
      [
        session({ id: "1", interfaceId: "rs" }),
        session({ id: "2", interfaceId: null }),
        session({ id: "3", interfaceId: "rs" }),
      ],
      [vlan],
    );
    expect(groups).toHaveLength(2);
    expect(groups[0].iface?.id).toBe("rs");
    expect(groups[0].sessions.map((s) => s.id)).toEqual(["1", "3"]);
    expect(groups[1].iface).toBeNull();
  });
});

describe("groupInterfacesByParent", () => {
  it("lists VLANs under their physical port, and the port itself with them", () => {
    const groups = groupInterfacesByParent([
      iface({ id: "a", name: "vlan-b", parentName: "sfp1" }),
      iface({ id: "b", name: "sfp1" }),
      iface({ id: "c", name: "vlan-a", parentName: "sfp1" }),
      iface({ id: "d", name: "ether2" }),
    ]);
    expect(groups.map((g) => g.parent)).toEqual(["ether2", "sfp1"]);
    expect(groups[1].interfaces.map((i) => i.name)).toEqual([
      "sfp1",
      "vlan-a",
      "vlan-b",
    ]);
  });
});

describe("filterInterfaces", () => {
  it("matches name or comment, ignoring case", () => {
    const list = [
      iface({ id: "1", name: "vlan-1113", comment: "Cariu Via Fibernet" }),
      iface({ id: "2", name: "vlan-1119", comment: "Depok" }),
    ];
    expect(filterInterfaces(list, "cariu").map((i) => i.id)).toEqual(["1"]);
    expect(filterInterfaces(list, "1119").map((i) => i.id)).toEqual(["2"]);
    expect(filterInterfaces(list, "  ")).toHaveLength(2);
  });
});

describe("stackSeries", () => {
  it("lines each upstream up by timestamp for a stacked chart", () => {
    const a: RouterTrafficPoint[] = [
      { time: "2026-10-12T01:00:00Z", inBps: 10, outBps: 1 },
      { time: "2026-10-12T01:01:00Z", inBps: 20, outBps: 2 },
    ];
    const b: RouterTrafficPoint[] = [
      { time: "2026-10-12T01:01:00Z", inBps: 5, outBps: 0 },
    ];
    expect(stackSeries({ Transit: a, IX: b }, "inBps")).toEqual([
      { time: "2026-10-12T01:00:00Z", Transit: 10 },
      { time: "2026-10-12T01:01:00Z", Transit: 20, IX: 5 },
    ]);
  });
});

describe("interfaceSeries", () => {
  it("names the two directions with the labels it is given", () => {
    expect(
      interfaceSeries([{ time: "t", inBps: 3, outBps: 4 }], {
        in: "Download",
        out: "Upload",
      }),
    ).toEqual([{ time: "t", Download: 3, Upload: 4 }]);
  });
});

describe("formatting", () => {
  it("formats bits per second in the unit that keeps them readable", () => {
    expect(formatBps(642_100_000)).toBe("642.10 Mbps");
    expect(formatBps(1_500_000_000)).toBe("1.50 Gbps");
    expect(formatBps(null)).toBe("—");
  });

  it("formats an uptime in days, hours and minutes", () => {
    expect(formatUptime(38 * 3600 + 52 * 60 + 22)).toBe("1h 14j 52m");
    expect(formatUptime(15 * 3600 + 53 * 60)).toBe("15j 53m");
    expect(formatUptime(0)).toBe("—");
  });

  it("computes memory used from total and free", () => {
    expect(memoryUsedPercent(16, 12)).toBe(25);
    expect(memoryUsedPercent(null, 12)).toBeNull();
    expect(memoryUsedPercent(0, 0)).toBeNull();
  });
});

describe("interfaceStatus", () => {
  it("reads a VLAN the router calls up but that receives nothing as down", () => {
    const status = interfaceStatus(
      iface({ operStatus: "up", silentSince: "2026-10-12T11:55:45Z" }),
    );

    expect(status.color).toBe("red");
    expect(status.text).toMatch(
      /^Tidak ada trafik masuk sejak .+ — kemungkinan down$/,
    );
  });

  it("keeps the router's own word otherwise", () => {
    expect(interfaceStatus(iface({ operStatus: "up" }))).toEqual({
      text: "Up",
      color: "green",
    });
    expect(interfaceStatus(iface({ operStatus: "down" })).text).toBe("Down");
    expect(
      interfaceStatus(
        iface({ operStatus: "down", silentSince: "2026-10-12T11:55:45Z" }),
      ).text,
    ).toBe("Down");
    expect(
      interfaceStatus(
        iface({ disabled: true, silentSince: "2026-10-12T11:55:45Z" }),
      ).text,
    ).toBe("Disabled");
  });
});
