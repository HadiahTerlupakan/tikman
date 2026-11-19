import dayjs from "dayjs";
import type { BrasDrop, BrasSample } from "@/domain/entities";
import { formatCount } from "./brasSignalModel";

export interface SessionsPoint {
  t: number;
  sessions: number | null;
}

export interface DropPoint {
  t: number;
  sessions: number;
  label: string;
}

const MINUTE_MS = 60_000;
// The worker samples every minute; a gap wider than two means minutes were
// missed, and drawing straight across them would invent a count.
const MAX_GAP_MS = 2 * MINUTE_MS;

/** Samples as chart points, with a null point inside each gap so the line breaks. */
export function sessionsSeries(samples: BrasSample[]): SessionsPoint[] {
  const points: SessionsPoint[] = [];
  let prev: number | null = null;
  for (const s of samples) {
    const t = Date.parse(s.time);
    if (prev !== null && t - prev > MAX_GAP_MS) {
      points.push({ t: prev + MINUTE_MS, sessions: null });
    }
    points.push({ t, sessions: s.sessions });
    prev = t;
  }
  return points;
}

export function dropLabel(drop: BrasDrop): string {
  return `${dayjs(drop.time).format("HH:mm")} turun ${formatCount(drop.lost)} sesi`;
}

/** Each drop placed on the line at the sample it was found in. */
export function dropPoints(
  drops: BrasDrop[],
  samples: BrasSample[],
): DropPoint[] {
  const byTime = new Map(samples.map((s) => [Date.parse(s.time), s.sessions]));
  return drops.flatMap((d) => {
    const t = Date.parse(d.time);
    const sessions = byTime.get(t);
    return sessions === undefined ? [] : [{ t, sessions, label: dropLabel(d) }];
  });
}
