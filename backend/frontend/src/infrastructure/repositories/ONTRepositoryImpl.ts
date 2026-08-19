import { apiClient } from '../http/apiClient';
import {
  ONT,
  CreateONTPayload,
  UpdateONTPayload,
} from '../../domain/entities/ONT';
import {
  ONTRepository,
  ONTListParams,
  ONTListResponse,
} from '../../domain/repositories/ONTRepository';

export class ONTRepositoryImpl implements ONTRepository {
  async list(params?: ONTListParams): Promise<ONTListResponse> {
    const response = await apiClient.get<ONTListResponse>('/onts', { params });
    return response.data;
  }

  async getById(id: string): Promise<ONT> {
    const response = await apiClient.get<ONT>(`/onts/${id}`);
    return response.data;
  }

  async create(payload: CreateONTPayload): Promise<ONT> {
    const response = await apiClient.post<ONT>('/onts', payload);
    return response.data;
  }

  async update(id: string, payload: UpdateONTPayload): Promise<ONT> {
    const response = await apiClient.put<ONT>(`/onts/${id}`, payload);
    return response.data;
  }

  async delete(id: string): Promise<void> {
    await apiClient.delete(`/onts/${id}`);
  }
}

export const ontRepository = new ONTRepositoryImpl();
