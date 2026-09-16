export type WaitEndReason = "replied" | "phone" | "closed" | "abandoned";

/** How fast a set of waits was answered, in minutes. A null figure means there
 * was nothing to measure, which is not the same as zero. */
export interface WaitStats {
  count: number;
  medianMinutes: number | null;
  p90Minutes: number | null;
  withinTargetPct: number | null;
}

export interface TeamPerformance {
  replies: WaitStats;
  closedWithoutReply: number;
  abandoned: number;
  systemDelayed: number;
}

export interface AgentPerformance {
  userId: string;
  username: string;
  replies: WaitStats;
  closedWithoutReply: number;
}

export interface DayPerformance {
  /** A WIB calendar day, YYYY-MM-DD. */
  date: string;
  replies: WaitStats;
}

export interface PerformanceSummary {
  targetMinutes: number;
  team: TeamPerformance;
  agents: AgentPerformance[];
  days: DayPerformance[];
  waiting: { count: number; longestMinutes: number | null };
}

/** One customer's wait, as the list behind the figures shows it. */
export interface PerformanceWait {
  id: string;
  conversationId: string;
  customerName: string;
  customerPhone: string;
  /** The thread was deleted with its number; the wait outlives it. */
  conversationDeleted: boolean;
  startedAt: string;
  customerSentAt: string;
  endedAt: string;
  endReason: WaitEndReason;
  endedBy?: string;
  endedByUsername?: string;
  teamMinutes: number | null;
  countedMinutes: number | null;
  systemDelayed: boolean;
}

export interface PerformanceWaitPage {
  items: PerformanceWait[];
  total: number;
}

/** The inclusive WIB dates a report covers, as YYYY-MM-DD. */
export interface PerformancePeriod {
  from: string;
  to: string;
}

/** One page of the waits behind a report. userId narrows it to one CS; the
 * server ignores it for anyone but an admin. */
export interface PerformanceWaitQuery extends PerformancePeriod {
  userId?: string;
  limit: number;
  offset: number;
}
