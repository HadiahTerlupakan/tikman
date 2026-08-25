import type { OltStats } from "@/domain/entities";

export interface OltProgressDisplay {
  percent: number;
  label: string;
  count: string;
}

export function getOltProgressDisplay(stats: OltStats): OltProgressDisplay {
  const total = stats.discoveryTotal ?? 0;
  const registered = stats.discoveryRegistered ?? 0;
  const phase = stats.phase ?? "idle";

  if (phase === "discovering" && total === 0) {
    return { percent: 0, label: "Discovering ONTs…", count: "Waiting for OLT" };
  }

  if (phase === "discovering" || phase === "polling") {
    const percent = total > 0 ? Math.round((registered / total) * 100) : 0;
    return {
      percent,
      label: "Discovering ONTs",
      count: `${registered}/${total} ONTs found`,
    };
  }

  return {
    percent: Math.round(stats.percentage),
    label: "Polling metrics",
    count: `${stats.ontsWithMetrics}/${stats.totalOnts} ONTs polled`,
  };
}
