import { useQuery } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";

const ontRepository = new OntRepository();

export function useOntMetrics(
  id: string,
  enabled = true,
  pollingInterval?: number,
) {
  const interval = pollingInterval || 300000;

  return useQuery({
    queryKey: ["onts", id, "metrics", interval],
    queryFn: () => ontRepository.getLatestMetrics(id),
    enabled: enabled && !!id,
    refetchInterval: interval,
    refetchIntervalInBackground: true,
    retry: false,
  });
}

export function useOntMetricsHistory(id: string, start?: string, end?: string) {
  return useQuery({
    queryKey: ["onts", id, "metrics-history", start, end],
    queryFn: () => ontRepository.getMetricsHistory(id, start, end),
    enabled: !!id,
  });
}

// The OLT recomputes its octet-rate gauges on its own schedule, measured at
// about fifteen seconds: polling every three returned the identical value four
// or five times in a row, which is why the rate looked frozen. Matching the
// device shows a fresh figure on nearly every poll and asks the OLT for a fifth
// as much.
const REALTIME_POLL_INTERVAL = 15000;

export function useOntMetricsRealtime(id: string, enabled = true) {
  return useQuery({
    queryKey: ["onts", id, "metrics-realtime"],
    queryFn: () => ontRepository.getRealtimeMetrics(id),
    enabled: enabled && !!id,
    refetchInterval: REALTIME_POLL_INTERVAL,
    // Each poll is a live SNMP round trip to the OLT. Nobody is reading the
    // modal behind a hidden tab, so it stops rather than keeps the device busy.
    refetchIntervalInBackground: false,
    staleTime: 0,
    retry: false,
  });
}

// The worker collects metrics every 60s, so polling faster only re-reads the
// same rows from ont_metrics.
const TRAFFIC_TIMESERIES_POLL_INTERVAL = 60000;

export function useOntTrafficTimeSeries(
  id: string,
  period: string,
  range?: { start: string; end: string },
  enabled = true,
) {
  return useQuery({
    queryKey: [
      "onts",
      id,
      "traffic-timeseries",
      period,
      range?.start,
      range?.end,
    ],
    queryFn: () => ontRepository.getTrafficTimeSeries(id, period, range),
    enabled: enabled && !!id,
    refetchInterval: TRAFFIC_TIMESERIES_POLL_INTERVAL,
    refetchIntervalInBackground: true,
    staleTime: 0,
    retry: false,
  });
}
