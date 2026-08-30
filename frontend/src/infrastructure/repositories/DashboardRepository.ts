import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { DashboardStats } from "@/domain/entities";

export class DashboardRepository {
  async getStats(): Promise<DashboardStats> {
    const response = await apiClient.get<DashboardStats>(
      API_ENDPOINTS.DASHBOARD_STATS,
    );
    return response.data;
  }
}
