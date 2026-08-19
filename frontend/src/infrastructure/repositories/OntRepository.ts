import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IOntRepository } from "@/domain/repositories";
import type { Ont, CreateOntDto, UpdateOntDto, OntMetrics, ONTEventsResponse, AvailabilityStats } from "@/domain/entities";

export class OntRepository implements IOntRepository {
  async getAll(params?: {
    oltId?: string;
    status?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ data: Ont[]; total: number }> {
    const response = await apiClient.get(API_ENDPOINTS.ONTS, {
      params: {
        ...params,
        limit: params?.limit || 200,  // Get maximum for client-side pagination
        offset: params?.offset || 0,
      }
    });
    return response.data;
  }

  async getById(id: string): Promise<Ont> {
    const response = await apiClient.get(API_ENDPOINTS.ONT_BY_ID(id));
    return response.data;
  }

  async create(data: CreateOntDto): Promise<Ont> {
    const response = await apiClient.post(API_ENDPOINTS.ONTS, data);
    return response.data;
  }

  async update(id: string, data: UpdateOntDto): Promise<Ont> {
    const response = await apiClient.put(API_ENDPOINTS.ONT_BY_ID(id), data);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.ONT_BY_ID(id));
  }

  async getLatestMetrics(id: string): Promise<OntMetrics> {
    const response = await apiClient.get<OntMetrics>(
      API_ENDPOINTS.ONT_LATEST_METRICS(id)
    );
    return response.data;
  }

  async getMetricsHistory(
    id: string,
    start?: string,
    end?: string
  ): Promise<{ data: OntMetrics[]; start: string; end: string; count: number }> {
    const response = await apiClient.get(API_ENDPOINTS.ONT_METRICS_HISTORY(id), {
      params: { start, end },
    });
    return response.data;
  }

  async getEvents(id: string, limit = 50, offset = 0): Promise<ONTEventsResponse> {
    const response = await apiClient.get(API_ENDPOINTS.ONT_EVENTS(id), {
      params: { limit, offset },
    });
    return response.data;
  }

  async getAvailability(id: string, days = 7): Promise<AvailabilityStats> {
    const response = await apiClient.get(API_ENDPOINTS.ONT_AVAILABILITY(id), {
      params: { days },
    });
    return response.data;
  }
}
