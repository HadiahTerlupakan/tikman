import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IOntRepository } from "@/domain/repositories";
import type {
  Ont,
  CreateOntDto,
  UpdateOntDto,
  OntMetrics,
  ONTEventsResponse,
  AvailabilityStats,
  TopologySlotResponse,
} from "@/domain/entities";

export class OntRepository implements IOntRepository {
  async getAll(params?: {
    oltId?: string;
    status?: string;
    startTime?: string;
    endTime?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ data: Ont[]; total: number }> {
    const queryParams = {
      olt_id: params?.oltId,
      status: params?.status,
      start_time: params?.startTime,
      end_time: params?.endTime,
      limit: params?.limit || 200,
      offset: params?.offset || 0,
    };

    const response = await apiClient.get(API_ENDPOINTS.ONTS, {
      params: queryParams,
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
      API_ENDPOINTS.ONT_LATEST_METRICS(id),
    );
    return response.data;
  }

  async getMetricsHistory(
    id: string,
    start?: string,
    end?: string,
  ): Promise<{
    data: OntMetrics[];
    start: string;
    end: string;
    count: number;
  }> {
    const response = await apiClient.get(
      API_ENDPOINTS.ONT_METRICS_HISTORY(id),
      {
        params: { start, end },
      },
    );
    return response.data;
  }

  async getRealtimeMetrics(id: string): Promise<OntMetrics> {
    const response = await apiClient.get<OntMetrics>(
      API_ENDPOINTS.ONT_REALTIME_METRICS(id),
    );
    return response.data;
  }

  async getEvents(
    id: string,
    limit = 50,
    offset = 0,
  ): Promise<ONTEventsResponse> {
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

  async getTrafficTimeSeries(
    id: string,
    period: string,
    range?: { start: string; end: string; bucket?: "hour" | "day" | "month" },
  ): Promise<OntMetrics[]> {
    const params: Record<string, string> = range
      ? { start: range.start, end: range.end, bucket: range.bucket || "hour" }
      : { period };
    const response = await apiClient.get(API_ENDPOINTS.ONT_TIMESERIES(id), {
      params,
    });
    return response.data;
  }

  async getTopology(oID: string): Promise<TopologySlotResponse[]> {
    const response = await apiClient.get<{ topology: TopologySlotResponse[] }>(
      `${API_ENDPOINTS.OLTS}/${oID}/topology/cached`,
    );
    return response.data.topology ?? [];
  }
}
