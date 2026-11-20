import { describe, expect, it } from "vitest";
import type { SwitchInterface } from "@/domain/entities";
import {
  buildSwitchPortViews,
  physicalSwitchPorts,
  switchUnitStatus,
} from "./switchPorts";

const port = (over: Partial<SwitchInterface>): SwitchInterface =>
  ({
    id: over.name,
    switchId: "s-1",
    name: "10GE1/0/1",
    alias: "",
    kind: "sfp",
    adminUp: true,
    operStatus: "up",
    speedMbps: 10000,
    lastInBps: 2e6,
    lastOutBps: 1e6,
    ...over,
  }) as SwitchInterface;

describe("switch ports", () => {
  it("lists physical ports in panel order, without VLAN interfaces", () => {
    const names = physicalSwitchPorts([
      port({ name: "10GE1/0/10" }),
      port({ name: "Vlanif10", kind: "virtual" }),
      port({ name: "10GE1/0/2" }),
      port({ name: "Eth-Trunk1", kind: "virtual" }),
    ]);

    expect(names).toEqual(["10GE1/0/2", "10GE1/0/10"]);
  });

  it("draws a port by its link, and a dark one only when nothing claims it", () => {
    const views = buildSwitchPortViews([
      port({ name: "10GE1/0/1", alias: "** ke-olt **" }),
      port({ name: "10GE1/0/2", alias: "ke-pop", operStatus: "down" }),
      port({ name: "10GE1/0/3", operStatus: "down" }),
      port({ name: "10GE1/0/4", adminUp: false, operStatus: "down" }),
      port({ name: "MEth0/0/0", kind: "rj" }),
    ]);

    expect(views.get("10GE1/0/1")).toMatchObject({
      state: "up",
      role: "dist",
      label: "ke-olt",
      inBps: 2e6,
    });
    expect(views.get("10GE1/0/2")).toMatchObject({
      state: "down",
      role: "dist",
    });
    expect(views.get("10GE1/0/3")?.role).toBeNull();
    expect(views.get("10GE1/0/4")).toMatchObject({
      state: "disabled",
      role: null,
    });
    expect(views.get("MEth0/0/0")?.role).toBe("mgmt");
  });

  it("names what needs a look on a unit", () => {
    const views = buildSwitchPortViews([
      port({ name: "10GE1/0/1" }),
      port({ name: "10GE1/0/2", alias: "ke-pop", operStatus: "down" }),
      port({ name: "10GE1/0/3", operStatus: "down" }),
    ]);
    const names = [...views.keys()];
    const ok = { loading: false, failed: false };

    expect(switchUnitStatus(true, views, names, ok)).toEqual({
      text: "1 port mati",
      tone: "warn",
    });
    expect(switchUnitStatus(false, views, names, ok)?.tone).toBe("down");
    expect(
      switchUnitStatus(true, views, names, { loading: true, failed: false }),
    ).toBeNull();
    expect(switchUnitStatus(true, new Map(), [], ok)?.text).toBe(
      "Belum ada data interface",
    );
  });
});
