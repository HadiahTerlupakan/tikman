import type {
  ManagedSwitch,
  RouterInterface,
  SwitchInterface,
  SwitchVendor,
} from "@/domain/entities";
import {
  FIBERHOME_KINDS,
  HUAWEI_CE_KINDS,
  ROUTEROS_KINDS,
  type PortKinds,
} from "../routers/faceplate/portLayout";
import type { PortState, PortView } from "../routers/faceplate/portModel";

export const SWITCH_PORT_KINDS: Record<SwitchVendor, PortKinds> = {
  huawei_ce: HUAWEI_CE_KINDS,
  fiberhome: FIBERHOME_KINDS,
  crs: ROUTEROS_KINDS,
};

const MGMT_PORT = /^MEth/;

export interface SwitchUnitStatus {
  text: string;
  tone: "" | "warn" | "down" | "none";
}

export function switchReachable(sw: ManagedSwitch): boolean {
  return sw.enabled && sw.status === "up";
}

/** Physical ports only: trunks and VLAN interfaces have no jack. */
export function physicalSwitchPorts(ports: SwitchInterface[]): string[] {
  return ports
    .filter((p) => p.kind !== "virtual")
    .map((p) => p.name)
    .sort((a, b) => a.localeCompare(b, "en", { numeric: true }));
}

function portState(p: SwitchInterface): PortState {
  if (!p.adminUp) return "disabled";
  return p.operStatus === "up" ? "up" : "down";
}

// The chassis draws router interfaces; a switch port carries the fields it reads.
function asChassisInterface(p: SwitchInterface): RouterInterface {
  return {
    id: p.id,
    routerId: p.switchId,
    name: p.name,
    comment: p.alias,
    type: "ether",
    vlanId: null,
    parentName: null,
    ifIndex: p.ifIndex,
    speedMbps: p.speedMbps,
    disabled: !p.adminUp,
    operStatus: p.operStatus,
    addresses: "",
    lastInBps: p.lastInBps,
    lastOutBps: p.lastOutBps,
    lastPolledAt: p.lastPolledAt,
    lastTrafficAt: null,
    silentSince: null,
  };
}

// A switch has no BGP to say what a port is for, so a port counts as used
// while its link is up or an operator described it; a down port nobody
// described is an empty cage, not a fault.
function portView(p: SwitchInterface): PortView {
  const state = portState(p);
  const mgmt = MGMT_PORT.test(p.name);
  const used = mgmt || (p.adminUp && (state === "up" || p.alias !== ""));
  const role = !used ? null : mgmt ? "mgmt" : "dist";
  return {
    iface: asChassisInterface(p),
    label: p.alias || p.name,
    state,
    role,
    roles: role ? [role] : [],
    vlans: [],
    sessions: [],
    deadSessions: 0,
    inBps: p.lastInBps ?? 0,
    outBps: p.lastOutBps ?? 0,
  };
}

export function buildSwitchPortViews(
  ports: SwitchInterface[],
): Map<string, PortView> {
  return new Map(ports.map((p) => [p.name, portView(p)]));
}

/** null while ports load, so nothing is claimed before there is data. */
export function switchUnitStatus(
  reachable: boolean,
  views: Map<string, PortView>,
  physical: string[],
  query: { loading: boolean; failed: boolean },
): SwitchUnitStatus | null {
  if (!reachable) return { text: "Tidak terjangkau", tone: "down" };
  if (query.loading) return null;
  if (query.failed) return { text: "Data port gagal dimuat", tone: "warn" };
  if (physical.length === 0) {
    return { text: "Belum ada data interface", tone: "none" };
  }
  const down = physical.filter((name) => {
    const view = views.get(name);
    return view?.role != null && view.state === "down";
  }).length;
  return down
    ? { text: `${down} port mati`, tone: "warn" }
    : { text: "Semua normal", tone: "" };
}
