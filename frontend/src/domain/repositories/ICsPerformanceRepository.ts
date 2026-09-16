import type {
  PerformancePeriod,
  PerformanceSummary,
  PerformanceWaitPage,
  PerformanceWaitQuery,
} from "@/domain/entities";

/** Reads the CS response-time report. */
export interface ICsPerformanceRepository {
  getSummary(period: PerformancePeriod): Promise<PerformanceSummary>;
  getWaits(query: PerformanceWaitQuery): Promise<PerformanceWaitPage>;
}
