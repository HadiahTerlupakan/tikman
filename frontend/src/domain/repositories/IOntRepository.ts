import type { Ont, CreateOntDto, UpdateOntDto, OntMetrics } from "../entities";

export interface IOntRepository {
  getAll(params?: {
    oltId?: string;
    status?: string;
    limit?: number;
    offset?: number;
  }): Promise<{ data: Ont[]; total: number }>;
  getById(id: string): Promise<Ont>;
  create(data: CreateOntDto): Promise<Ont>;
  update(id: string, data: UpdateOntDto): Promise<Ont>;
  delete(id: string): Promise<void>;
  getLatestMetrics(id: string): Promise<OntMetrics>;
  getMetricsHistory(
    id: string,
    start?: string,
    end?: string
  ): Promise<{
    data: OntMetrics[];
    start: string;
    end: string;
    count: number;
  }>;
}
