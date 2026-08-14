import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query';
import { OltRepository } from '@/infrastructure/repositories';
import type { CreateOltDto, UpdateOltDto } from '@/domain/entities';

const oltRepository = new OltRepository();

export function useOlts(siteId?: string) {
  return useQuery({
    queryKey: siteId ? ['olts', 'site', siteId] : ['olts'],
    queryFn: () => (siteId ? oltRepository.getBySite(siteId) : oltRepository.getAll()),
  });
}

export function useOlt(id: string) {
  return useQuery({
    queryKey: ['olts', id],
    queryFn: () => oltRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateOltDto) => oltRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}

export function useUpdateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateOltDto }) =>
      oltRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['olts', variables.id] });
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}

export function useDeleteOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => oltRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['olts'] });
      queryClient.invalidateQueries({ queryKey: ['sites'] });
    },
  });
}
