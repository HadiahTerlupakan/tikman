import dayjs from "dayjs";
import type { BrasSnapshot, RouterBrasView } from "@/domain/entities";

export type SignalLevel = "ok" | "warn" | "bad" | "none";

export interface Signal {
  level: SignalLevel;
  value: string;
  context: string;
}

export interface BrasSignalSet {
  sessions: Signal;
  radius: Signal;
  cpu: Signal;
  pool: Signal;
}

const MINUTE_MS = 60_000;
const MINUTES_PER_HOUR = 60;
const HOURS_PER_DAY = 24;
const DROP_WINDOW_MS = 15 * MINUTE_MS;
const STALE_AFTER_MS = 5 * MINUTE_MS;
const CPU_WARN = 70;
const CPU_BAD = 90;
const POOL_WARN_FREE = 0.1;
const POOL_BAD_FREE = 0.02;
const REJECT_WARN_SHARE = 0.1;
const PERCENT = 100;
const DASH = "—";
const BLANK: Signal = { level: "none", value: DASH, context: DASH };

export function formatCount(n: number | null | undefined): string {
  return n === null || n === undefined ? DASH : n.toLocaleString("id-ID");
}

/** A reading older than five minutes means the poller stopped, not that all is well. */
export function isBrasStale(brasAt: string | null, now: Date): boolean {
  return brasAt !== null && now.getTime() - Date.parse(brasAt) > STALE_AFTER_MS;
}

/** How long ago a reading was, in Indonesian; older than a day, its date. */
export function sinceText(at: string, now: Date): string {
  const minutes = Math.floor((now.getTime() - Date.parse(at)) / MINUTE_MS);
  if (minutes < MINUTES_PER_HOUR) return `${minutes} menit lalu`;
  const hours = Math.floor(minutes / MINUTES_PER_HOUR);
  if (hours < HOURS_PER_DAY) return `${hours} jam lalu`;
  return dayjs(at).format("DD MMM HH:mm");
}

function sessionsSignal(view: RouterBrasView, snap: BrasSnapshot): Signal {
  const at = Date.parse(view.brasAt ?? "");
  const recentDrop = view.drops.some(
    (d) => at - Date.parse(d.time) <= DROP_WINDOW_MS,
  );
  return {
    level: recentDrop ? "bad" : "ok",
    value: formatCount(snap.sessions),
    context: `15 menit lalu ${formatCount(view.sessions15mAgo)}`,
  };
}

function radiusWarn(view: RouterBrasView, silent: number): boolean {
  if (silent > 0 || (view.radiusTimeouts1h ?? 0) > 0) return true;
  const { radiusRejects1h: rejects, radiusAccepts1h: accepts } = view;
  return (
    rejects !== null &&
    accepts !== null &&
    rejects > REJECT_WARN_SHARE * accepts
  );
}

/**
 * A BRAS with no RADIUS entry logs users in from local secrets: grey, never
 * red. A server with answering null has no verdict yet and counts as neither.
 */
function radiusSignal(view: RouterBrasView, snap: BrasSnapshot): Signal {
  const servers = snap.radius;
  if (servers.length === 0) {
    return {
      level: "none",
      value: "Tidak dipakai",
      context: "Tanpa server RADIUS",
    };
  }
  const silent = servers.filter((s) => s.answering === false).length;
  const context = `${formatCount(view.radiusRejects1h)} login ditolak dalam 1 jam`;
  if (silent === servers.length) {
    return { level: "bad", value: "Tidak menjawab", context };
  }
  const value =
    silent === 0
      ? "Menjawab"
      : `${servers.length - silent} dari ${servers.length} menjawab`;
  return { level: radiusWarn(view, silent) ? "warn" : "ok", value, context };
}

function cpuSignal(snap: BrasSnapshot): Signal {
  const loads = snap.cores.map((c) => c.load);
  if (loads.length === 0) return BLANK;
  const max = Math.max(...loads);
  const avg = Math.round(loads.reduce((a, b) => a + b, 0) / loads.length);
  const level = max >= CPU_BAD ? "bad" : max >= CPU_WARN ? "warn" : "ok";
  return {
    level,
    value: `${max}%`,
    context: `rata-rata ${loads.length} core ${avg}%`,
  };
}

/** A pool's level by the share of its addresses still free. */
export function poolLevel(free: number, total: number): SignalLevel {
  const share = free / total;
  return share < POOL_BAD_FREE ? "bad" : share < POOL_WARN_FREE ? "warn" : "ok";
}

/** Pools whose total is unknown (not plain IPv4) cannot be judged and are left out. */
function poolSignal(snap: BrasSnapshot): Signal {
  const known = snap.pools.flatMap((p) =>
    p.total !== null && p.free !== null && p.total > 0
      ? [{ name: p.name, total: p.total, free: p.free }]
      : [],
  );
  if (known.length === 0) return BLANK;
  const fewest = Math.min(...known.map((p) => p.free));
  const fullest = known.reduce((a, b) =>
    b.free / b.total < a.free / a.total ? b : a,
  );
  const usedPct = Math.round(
    ((fullest.total - fullest.free) / fullest.total) * PERCENT,
  );
  return {
    level: poolLevel(fullest.free, fullest.total),
    value: formatCount(fewest),
    context: `${fullest.name} terpakai ${usedPct}%`,
  };
}

/** The Ringkasan tab's four signals; a stale reading keeps its values but loses its colours. */
export function brasSignals(view: RouterBrasView, now: Date): BrasSignalSet {
  const snap = view.bras;
  if (!snap || !view.brasAt) {
    return { sessions: BLANK, radius: BLANK, cpu: BLANK, pool: BLANK };
  }
  const signals: BrasSignalSet = {
    sessions: sessionsSignal(view, snap),
    radius: radiusSignal(view, snap),
    cpu: cpuSignal(snap),
    pool: poolSignal(snap),
  };
  if (!isBrasStale(view.brasAt, now)) return signals;
  const grey = (s: Signal): Signal => ({ ...s, level: "none" });
  return {
    sessions: grey(signals.sessions),
    radius: grey(signals.radius),
    cpu: grey(signals.cpu),
    pool: grey(signals.pool),
  };
}
