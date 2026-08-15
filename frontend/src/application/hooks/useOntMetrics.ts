import { useQuery } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";

const ontRepository = new OntRepository();

export function useOntMetrics(id: string, enabled = true) {
  return useQuery({
    queryKey: ["onts", id, "metrics"],
    queryFn: () => ontRepository.getLatestMetrics(id),
    enabled: enabled && !!id,
    refetchInterval: 300000, // 5 minutes
    retry: false, // Don't retry if no metrics available yet
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
