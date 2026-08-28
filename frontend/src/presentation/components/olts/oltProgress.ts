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

  // Only an OLT TikMan holds nothing for is genuinely waiting. Starting a poll
  // resets discoveryTotal to zero before the status walk reports it, so this
  // branch used to blank the bar of a fully polled OLT at the top of every
  // cycle.
  const nothingKnown = stats.totalOnts === 0;
  if (
    (phase === "idle" || phase === "discovering") &&
    total === 0 &&
    nothingKnown
  ) {
    return { percent: 0, label: "Discovering ONTs…", count: "Waiting for OLT" };
  }

  // Discovery progress is only worth showing while the inventory is still
  // being built. A re-poll of an OLT TikMan already holds in full restarts
  // that counter from zero, and preferring it there blanked a bar whose real
  // figure was 200 of 200 ONTs polled — which is what made an established OLT
  // read 0% for most of every cycle.
  const inventoryIncomplete = stats.totalOnts < total;
  if ((phase === "discovering" || phase === "polling") && inventoryIncomplete) {
    const percent = total > 0 ? Math.round((registered / total) * 100) : 0;
    return {
      percent,
      label: "Discovering ONTs",
      count: `${registered}/${total} ONTs found`,
    };
  }

  // Computed from the two numbers shown rather than read from stats.percentage,
  // which the server overrides with discovery progress whenever a poll is in
  // flight. The bar then read 0% next to a count of 200/200.
  const polled =
    stats.totalOnts > 0
      ? Math.round((stats.ontsWithMetrics / stats.totalOnts) * 100)
      : 0;

  return {
    percent: polled,
    label: "Polling metrics",
    count: `${stats.ontsWithMetrics}/${stats.totalOnts} ONTs polled`,
  };
}
