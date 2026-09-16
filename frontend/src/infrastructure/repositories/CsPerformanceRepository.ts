import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { ICsPerformanceRepository } from "@/domain/repositories";
import type {
  PerformancePeriod,
  PerformanceSummary,
  PerformanceWaitPage,
  PerformanceWaitQuery,
} from "@/domain/entities";

/**
 * CsPerformanceRepository reads the CS response-time report. Query params are
 * written in snake_case by hand: the client only decamelizes request bodies.
 */
export class CsPerformanceRepository implements ICsPerformanceRepository {
  async getSummary(period: PerformancePeriod): Promise<PerformanceSummary> {
    const response = await apiClient.get(API_ENDPOINTS.CS_PERFORMANCE_SUMMARY, {
      params: { from: period.from, to: period.to },
    });
    return response.data.data;
  }

  async getWaits(query: PerformanceWaitQuery): Promise<PerformanceWaitPage> {
    const response = await apiClient.get(API_ENDPOINTS.CS_PERFORMANCE_WAITS, {
      params: {
        from: query.from,
        to: query.to,
        user_id: query.userId,
        limit: query.limit,
        offset: query.offset,
      },
    });
    return { items: response.data.data ?? [], total: response.data.total ?? 0 };
  }
}
