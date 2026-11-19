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
  role: PortRole;
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

export function pollHost(r: MikrotikRouter): string {
  return r.host;
}

function holdsHost(iface: RouterInterface, host: string): boolean {
  return iface.addresses
    .split(",")
    .some((cidr) => cidr.trim().split("/")[0] === host);
}

function ownRole(
  iface: RouterInterface,
  sessions: RouterBgpSession[],
  host: string,
): PortRole | null {
  if (sessions.length > 0) return "upstream";
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
  host: string,
): VlanView[] {
  const views: VlanView[] = [];
  for (const iface of vlans) {
    const own = sessionsOf(iface, sessions);
    const role = ownRole(iface, own, host);
    if (!role) continue;
    views.push({
      iface,
      label: interfaceLabel(iface),
      role,
      inBps: iface.lastInBps ?? 0,
      sessions: own,
    });
  }
  return views;
}

function buildPort(
  iface: RouterInterface,
  vlans: VlanView[],
  all: RouterBgpSession[],
  host: string,
): PortView {
  const own = sessionsOf(iface, all);
  const base = ownRole(iface, own, host);
  // A trunk's own "dist" is just its link being up; only VLANs can make it one.
  const portRole = vlans.length > 0 && base === "dist" ? null : base;
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
  const host = pollHost(r);
  const views = new Map<string, PortView>();
  for (const iface of interfaces) {
    if (iface.parentName) continue;
    const children = interfaces.filter((i) => i.parentName === iface.name);
    views.set(
      iface.name,
      buildPort(iface, buildVlans(children, sessions, host), sessions, host),
    );
  }
  return views;
}
