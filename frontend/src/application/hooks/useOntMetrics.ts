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
