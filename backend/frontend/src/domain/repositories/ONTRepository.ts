import { ONT, CreateONTPayload, UpdateONTPayload, ONTStatus } from '../entities/ONT';

export interface ONTListParams {
  oltId?: string;
  status?: ONTStatus;
  limit?: number;
  offset?: number;
}

export interface ONTListResponse {
  data: ONT[];
  total: number;
  limit: number;
  offset: number;
}

export interface ONTRepository {
  list(params?: ONTListParams): Promise<ONTListResponse>;
  getById(id: string): Promise<ONT>;
  create(payload: CreateONTPayload): Promise<ONT>;
  update(id: string, payload: UpdateONTPayload): Promise<ONT>;
  delete(id: string): Promise<void>;
}
