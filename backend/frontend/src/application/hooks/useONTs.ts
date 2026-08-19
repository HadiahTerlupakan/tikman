import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { ontRepository } from '../../infrastructure/repositories/ONTRepositoryImpl';
import {
  ONTListParams,
  ONTListResponse,
} from '../../domain/repositories/ONTRepository';
import { ONT, CreateONTPayload, UpdateONTPayload } from '../../domain/entities/ONT';
import { message } from 'antd';

export const useONTs = (params?: ONTListParams) => {
  return useQuery<ONTListResponse>({
    queryKey: ['onts', params],
    queryFn: () => ontRepository.list(params),
    refetchInterval: 30000, // Auto-refresh every 30 seconds for real-time status
  });
};

export const useONT = (id: string) => {
  return useQuery<ONT>({
    queryKey: ['onts', id],
    queryFn: () => ontRepository.getById(id),
    enabled: !!id,
  });
};

export const useCreateONT = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (payload: CreateONTPayload) => ontRepository.create(payload),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['onts'] });
      message.success('ONT created successfully');
    },
    onError: (error: any) => {
      message.error(error.response?.data?.error || 'Failed to create ONT');
    },
  });
};

export const useUpdateONT = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, payload }: { id: string; payload: UpdateONTPayload }) =>
      ontRepository.update(id, payload),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['onts'] });
      queryClient.invalidateQueries({ queryKey: ['onts', variables.id] });
      message.success('ONT updated successfully');
    },
    onError: (error: any) => {
      message.error(error.response?.data?.error || 'Failed to update ONT');
    },
  });
};

export const useDeleteONT = () => {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => ontRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['onts'] });
      message.success('ONT deleted successfully');
    },
    onError: (error: any) => {
      message.error(error.response?.data?.error || 'Failed to delete ONT');
    },
  });
};
