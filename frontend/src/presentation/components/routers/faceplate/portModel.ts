import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
} from "@/domain/entities";
import { interfaceLabel } from "../routerTraffic";

export type PortRole = "upstream" | "dist" | "mgmt";
export type PortState = "up" | "down" | "silent" | "disabled";

export interface VlanView {
  iface: RouterInterface;
  label: string;
  /** null for a VLAN with no comment, session or link: listed, but drawn without a stripe. */
  role: PortRole | null;
  inBps: number;
  sessions: RouterBgpSession[];
}

export interface PortView {
  iface: RouterInterface;
  label: string;
  state: PortState;
  /** null means unused, drawn dark. */
  role: PortRole | "trunk" | null;
  /** Plug stripes, one per distinct role. */
  roles: PortRole[];
  vlans: VlanView[];
  /** The port's own sessions and those of every VLAN on it. */
  sessions: RouterBgpSession[];
  deadSessions: number;
  inBps: number;
  outBps: number;
}

export const STATE_TEXT: Record<PortState, string> = {
  up: "Aktif",
  down: "Mati",
  silent: "Kemungkinan down",
  disabled: "Nonaktif",
};

const ROLE_ORDER: PortRole[] = ["upstream", "dist", "mgmt"];

export function routerReachable(r: MikrotikRouter): boolean {
  return r.enabled && r.status === "up";
}

/** A reachable router whose API failed: its BGP sessions are last-known, not live. */
export function bgpStale(r: MikrotikRouter): boolean {
  return routerReachable(r) && r.apiStatus !== "ok";
}

function holdsHost(iface: RouterInterface, host: string): boolean {
  return iface.addresses
    .split(",")
    .some((cidr) => cidr.trim().split("/")[0] === host);
}

// A BRAS has no BGP to mark its way out, so the operator's chosen uplink does it.
function ownRole(
  iface: RouterInterface,
  sessions: RouterBgpSession[],
  r: MikrotikRouter,
): PortRole | null {
  const host = r.host;
  if (sessions.length > 0) return "upstream";
  if (r.role === "bras" && iface.id === r.uplinkInterfaceId) return "upstream";
  if (holdsHost(iface, host)) return "mgmt";
  if (iface.disabled) return null;
  return iface.comment.trim() || iface.operStatus === "up" ? "dist" : null;
}

function portState(iface: RouterInterface): PortState {
  if (iface.disabled) return "disabled";
  if (iface.operStatus === "down") return "down";
  return iface.silentSince ? "silent" : "up";
}

function distinctRoles(roles: (PortRole | null)[]): PortRole[] {
  return ROLE_ORDER.filter((role) => roles.includes(role));
}

function sessionsOf(
  iface: RouterInterface,
  sessions: RouterBgpSession[],
): RouterBgpSession[] {
  return sessions.filter((s) => s.interfaceId === iface.id);
}

function buildVlans(
  vlans: RouterInterface[],
  sessions: RouterBgpSession[],
  r: MikrotikRouter,
): VlanView[] {
  return vlans.map((iface) => {
    const own = sessionsOf(iface, sessions);
    return {
      iface,
      label: interfaceLabel(iface),
      role: ownRole(iface, own, r),
      inBps: iface.lastInBps ?? 0,
      sessions: own,
    };
  });
}

function buildPort(
  iface: RouterInterface,
  vlans: VlanView[],
  all: RouterBgpSession[],
  r: MikrotikRouter,
): PortView {
  const own = sessionsOf(iface, all);
  const base = ownRole(iface, own, r);
  // A trunk's own "dist" is just its link being up; only VLANs can make it one.
  const carries = vlans.some((v) => v.role !== null);
  const portRole = carries && base === "dist" ? null : base;
  const roles = distinctRoles([portRole, ...vlans.map((v) => v.role)]);
  const sessions = [...own, ...vlans.flatMap((v) => v.sessions)];
  return {
    iface,
    label: interfaceLabel(iface),
    state: portState(iface),
    role: roles.length > 1 ? "trunk" : roles[0] ?? null,
    roles,
    vlans,
    sessions,
    deadSessions: sessions.filter((s) => s.state !== "established").length,
    inBps: iface.lastInBps ?? 0,
    outBps: iface.lastOutBps ?? 0,
  };
}

/** One view per physical port, keyed by port name; VLANs fold into their parent. */
export function buildPortViews(
  r: MikrotikRouter,
  interfaces: RouterInterface[],
  sessions: RouterBgpSession[],
): Map<string, PortView> {
  const views = new Map<string, PortView>();
  for (const iface of interfaces) {
    if (iface.parentName) continue;
    const children = interfaces.filter((i) => i.parentName === iface.name);
    views.set(
      iface.name,
      buildPort(iface, buildVlans(children, sessions, r), sessions, r),
    );
  }
  return views;
}
