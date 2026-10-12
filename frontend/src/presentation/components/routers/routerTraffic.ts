import type {
  MikrotikRouter,
  RouterBgpSession,
  RouterInterface,
  RouterTrafficPoint,
} from "@/domain/entities";
import dayjs from "dayjs";
import { formatSpeed } from "../trafficFormat";

const BITS_PER_MEGABIT = 1_000_000;
const SECONDS_PER_DAY = 86_400;
const SECONDS_PER_HOUR = 3_600;
const SECONDS_PER_MINUTE = 60;

export interface DirectionLabels {
  in: string;
  out: string;
}

export interface SessionGroup {
  iface: RouterInterface | null;
  sessions: RouterBgpSession[];
}

export interface InterfaceGroup {
  parent: string;
  interfaces: RouterInterface[];
}

export type ChartRow = Record<string, string | number>;

/**
 * What the Interface & VLAN tab says about an interface. A VLAN to a POP
 * stays up while the site behind the carrier is gone, so silence outranks
 * the router's "up".
 */
export function interfaceStatus(iface: RouterInterface): {
  text: string;
  color?: string;
} {
  if (iface.disabled) return { text: "Disabled" };
  if (iface.silentSince)
    return {
      text: `Tidak ada trafik masuk sejak ${dayjs(iface.silentSince).format("DD MMM HH:mm")}`,
      color: "red",
    };
  if (iface.operStatus === "up") return { text: "Up", color: "green" };
  if (iface.operStatus === "down") return { text: "Down", color: "red" };
  return { text: "—" };
}

/** The comment is what operators named the link; the name is the fallback. */
export function interfaceLabel(iface: RouterInterface): string {
  return iface.comment.trim() || iface.name;
}

/** An interface carrying a BGP session is where traffic enters and leaves. */
export function isUpstream(
  iface: RouterInterface,
  sessions: RouterBgpSession[],
): boolean {
  return sessions.some((s) => s.interfaceId === iface.id);
}

/**
 * Names rx and tx from the side that matters. On an upstream the router
 * receives what subscribers download; on a POP VLAN the router transmits it,
 * so the same rx/tx would read backwards there.
 */
export function directionLabels(upstream: boolean): DirectionLabels {
  return upstream
    ? { in: "Download", out: "Upload" }
    : { in: "Dari pelanggan", out: "Ke pelanggan" };
}

/** Sessions under the interface they run over; unmatched ones in a last group. */
export function groupSessionsByInterface(
  sessions: RouterBgpSession[],
  interfaces: RouterInterface[],
): SessionGroup[] {
  const byId = new Map(interfaces.map((i) => [i.id, i]));
  const groups = new Map<string, SessionGroup>();
  const unmatched: RouterBgpSession[] = [];
  for (const s of sessions) {
    const iface = s.interfaceId ? byId.get(s.interfaceId) : undefined;
    if (!iface) {
      unmatched.push(s);
      continue;
    }
    const group = groups.get(iface.id) ?? { iface, sessions: [] };
    group.sessions.push(s);
    groups.set(iface.id, group);
  }
  const result = [...groups.values()];
  if (unmatched.length > 0) result.push({ iface: null, sessions: unmatched });
  return result;
}

/** VLANs under their physical port, the port listed first in its own group. */
export function groupInterfacesByParent(
  interfaces: RouterInterface[],
): InterfaceGroup[] {
  const groups = new Map<string, RouterInterface[]>();
  for (const iface of interfaces) {
    const parent = iface.parentName ?? iface.name;
    groups.set(parent, [...(groups.get(parent) ?? []), iface]);
  }
  return [...groups.entries()]
    .sort(([a], [b]) => a.localeCompare(b))
    .map(([parent, list]) => ({
      parent,
      interfaces: [...list].sort((a, b) => {
        if (a.name === parent) return -1;
        if (b.name === parent) return 1;
        return a.name.localeCompare(b.name);
      }),
    }));
}

export function filterInterfaces(
  interfaces: RouterInterface[],
  query: string,
): RouterInterface[] {
  const needle = query.trim().toLowerCase();
  if (!needle) return interfaces;
  return interfaces.filter(
    (i) =>
      i.name.toLowerCase().includes(needle) ||
      i.comment.toLowerCase().includes(needle),
  );
}

/** One row per timestamp, one column per series, for a stacked area chart. */
export function stackSeries(
  series: Record<string, RouterTrafficPoint[]>,
  field: "inBps" | "outBps",
): ChartRow[] {
  const rows = new Map<string, ChartRow>();
  for (const [label, points] of Object.entries(series)) {
    for (const point of points) {
      const row = rows.get(point.time) ?? { time: point.time };
      row[label] = point[field];
      rows.set(point.time, row);
    }
  }
  return [...rows.values()].sort((a, b) =>
    String(a.time).localeCompare(String(b.time)),
  );
}

export function interfaceSeries(
  points: RouterTrafficPoint[],
  labels: DirectionLabels,
): ChartRow[] {
  return points.map((p) => ({
    time: p.time,
    [labels.in]: p.inBps,
    [labels.out]: p.outBps,
  }));
}

export function formatBps(bps: number | null | undefined): string {
  if (bps === null || bps === undefined || !Number.isFinite(bps)) return "—";
  return formatSpeed(bps / BITS_PER_MEGABIT);
}

/** Days, hours and minutes in Indonesian short form: "1h 14j 52m". */
export function formatUptime(seconds: number | null | undefined): string {
  if (!seconds) return "—";
  const days = Math.floor(seconds / SECONDS_PER_DAY);
  const hours = Math.floor((seconds % SECONDS_PER_DAY) / SECONDS_PER_HOUR);
  const minutes = Math.floor((seconds % SECONDS_PER_HOUR) / SECONDS_PER_MINUTE);
  const parts = [
    days ? `${days}h` : "",
    hours ? `${hours}j` : "",
    `${minutes}m`,
  ];
  return parts.filter(Boolean).join(" ");
}

/** When the poller last wrote any of these sessions, null if it never has. */
export function lastBgpRead(sessions: RouterBgpSession[]): string | null {
  let newest: string | null = null;
  for (const s of sessions) {
    if (newest === null || Date.parse(s.updatedAt) > Date.parse(newest)) {
      newest = s.updatedAt;
    }
  }
  return newest;
}

/**
 * The BGP tally for the router list. Sessions are read over REST, so while REST
 * fails the stored tally is whatever it was last time and is not shown as now.
 */
export function bgpSummary(
  router: Pick<MikrotikRouter, "apiStatus" | "bgpEstablished" | "bgpTotal">,
): string {
  if (router.apiStatus !== "ok") return "—";
  return `${router.bgpEstablished}/${router.bgpTotal} Established`;
}

export function memoryUsedPercent(
  total: number | null,
  free: number | null,
): number | null {
  if (!total || free === null) return null;
  return Math.round(((total - free) / total) * 100);
}
