import { useQuery } from "@tanstack/react-query";
import { CsPerformanceRepository } from "@/infrastructure/repositories";
import type {
  PerformancePeriod,
  PerformanceWaitQuery,
} from "@/domain/entities";

const performanceRepository = new CsPerformanceRepository();

/** The team's, each CS's and each day's figures for one period. */
export function useCsPerformanceSummary(period: PerformancePeriod) {
  return useQuery({
    queryKey: ["cs", "performance", "summary", period],
    queryFn: () => performanceRepository.getSummary(period),
  });
}

/** One page of the waits behind the figures. Pass undefined while the list is
 * closed, so nothing is asked for until somebody opens it. */
export function useCsPerformanceWaits(query: PerformanceWaitQuery | undefined) {
  return useQuery({
    queryKey: ["cs", "performance", "waits", query],
    queryFn: () => performanceRepository.getWaits(query!),
    enabled: query !== undefined,
  });
}
