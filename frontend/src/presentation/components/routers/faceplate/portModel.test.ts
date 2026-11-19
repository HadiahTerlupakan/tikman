import { describe, expect, it } from "vitest";
import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import {
  bgpStale,
  buildPortViews,
  pollHost,
  routerReachable,
} from "./portModel";

const router = (over: Partial<MikrotikRouter> = {}): MikrotikRouter =>
  ({
    id: "r1",
    name: "border-1",
    host: "border.example",
    enabled: true,
    status: "up",
    apiStatus: "ok",
    ...over,
  }) as MikrotikRouter;

const iface = (over: Partial<RouterInterface>): RouterInterface => ({
  id: over.name ?? "i",
  routerId: "r1",
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

const session = (
  interfaceId: string | null,
  state: RouterBgpSession["state"] = "established",
): RouterBgpSession =>
  ({
    id: `s-${interfaceId}-${state}`,
    name: "peer-transit",
    remoteAddress: "192.0.2.9",
    state,
    interfaceId,
  }) as RouterBgpSession;

describe("buildPortViews", () => {
  it("marks a port carrying a session as upstream", () => {
    const ports = [iface({ name: "sfp-sfpplus1" })];
    const views = buildPortViews(router(), ports, [session("sfp-sfpplus1")]);
    const view = views.get("sfp-sfpplus1");
    expect(view?.role).toBe("upstream");
    expect(view?.sessions).toHaveLength(1);
  });

  it("makes a port with mixed VLAN roles a trunk", () => {
    const ports = [
      iface({ name: "sfp-sfpplus2" }),
      iface({ name: "vlan201", parentName: "sfp-sfpplus2", type: "vlan" }),
      iface({ name: "vlan101", parentName: "sfp-sfpplus2", type: "vlan" }),
    ];
    const view = buildPortViews(router(), ports, [session("vlan201")]).get(
      "sfp-sfpplus2",
    );
    expect(view?.role).toBe("trunk");
    expect(view?.roles).toEqual(["upstream", "dist"]);
    expect(view?.vlans.map((v) => [v.iface.name, v.role])).toEqual([
      ["vlan201", "upstream"],
      ["vlan101", "dist"],
    ]);
  });

  it("counts a down session behind a VLAN as a dead session of the port", () => {
    const ports = [
      iface({ name: "sfp-sfpplus2" }),
      iface({ name: "vlan201", parentName: "sfp-sfpplus2", type: "vlan" }),
    ];
    const view = buildPortViews(router(), ports, [
      session("vlan201", "down"),
    ]).get("sfp-sfpplus2");
    expect(view?.deadSessions).toBe(1);
    expect(view?.sessions).toHaveLength(1);
  });

  it("detects management only when an address equals the poll host", () => {
    const ports = [iface({ name: "ether1", addresses: "192.0.2.1/24" })];
    const hit = buildPortViews(router({ host: "192.0.2.1" }), ports, []);
    expect(hit.get("ether1")?.role).toBe("mgmt");
    const miss = buildPortViews(router(), ports, []);
    expect(miss.get("ether1")?.role).toBe("dist");
  });

  it("leaves a disabled uncommented port unused", () => {
    const ports = [iface({ name: "ether2", disabled: true })];
    const view = buildPortViews(router(), ports, []).get("ether2");
    expect(view?.role).toBeNull();
    expect(view?.state).toBe("disabled");
  });

  it("reports a silent port", () => {
    const ports = [iface({ name: "ether3", silentSince: "2026-01-01T00:00Z" })];
    expect(buildPortViews(router(), ports, []).get("ether3")?.state).toBe(
      "silent",
    );
  });

  it("ignores a session on an interface that is no port", () => {
    const ports = [iface({ name: "ether4", operStatus: "down" })];
    const view = buildPortViews(router(), ports, [session("bridge1")]).get(
      "ether4",
    );
    expect(view?.role).toBeNull();
    expect(view?.state).toBe("down");
    expect(view?.roles).toEqual([]);
  });
});

describe("router reachability", () => {
  it("is false for unknown, down and disabled routers", () => {
    expect(routerReachable(router({ status: "unknown" }))).toBe(false);
    expect(routerReachable(router({ status: "down" }))).toBe(false);
    expect(routerReachable(router({ enabled: false }))).toBe(false);
    expect(routerReachable(router())).toBe(true);
  });

  it("flags stale BGP data only while the router is reachable", () => {
    expect(bgpStale(router({ apiStatus: "error" }))).toBe(true);
    expect(bgpStale(router({ apiStatus: "error", status: "down" }))).toBe(
      false,
    );
    expect(bgpStale(router())).toBe(false);
  });

  it("exposes the poll host", () => {
    expect(pollHost(router({ host: "192.0.2.1" }))).toBe("192.0.2.1");
  });
});
