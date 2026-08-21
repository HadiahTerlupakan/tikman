import { useQuery } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";

const ontRepository = new OntRepository();

export function useOntMetrics(id: string, enabled = true, pollingInterval?: number) {
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

export function useOntMetricsHistory(
  id: string,
  start?: string,
  end?: string
) {
  return useQuery({
    queryKey: ["onts", id, "metrics-history", start, end],
    queryFn: () => ontRepository.getMetricsHistory(id, start, end),
    enabled: !!id,
  });
}

export function useOntMetricsRealtime(id: string, enabled = true) {
  return useQuery({
    queryKey: ["onts", id, "metrics-realtime"],
    queryFn: () => ontRepository.getRealtimeMetrics(id),
    enabled: enabled && !!id,
    refetchInterval: 3000,
    refetchIntervalInBackground: true,
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
  enabled = true
) {
  return useQuery({
    queryKey: ["onts", id, "traffic-timeseries", period, range?.start, range?.end],
    queryFn: () => ontRepository.getTrafficTimeSeries(id, period, range),
    enabled: enabled && !!id,
    refetchInterval: TRAFFIC_TIMESERIES_POLL_INTERVAL,
    refetchIntervalInBackground: true,
    staleTime: 0,
    retry: false,
  });
}

