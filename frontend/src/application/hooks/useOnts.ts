import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";
import type { CreateOntDto, UpdateOntDto, OntStatus } from "@/domain/entities";

const ontRepository = new OntRepository();

export function useOnts(params?: {
  oltId?: string;
  status?: OntStatus;
  limit?: number;
  offset?: number;
}) {
  return useQuery({
    queryKey: ["onts", params],
    queryFn: () => ontRepository.getAll(params),
    refetchInterval: 15000, // Refresh every 15 seconds for real-time status
    refetchIntervalInBackground: true, // Always refetch even when tab is not focused
    refetchOnWindowFocus: true, // Refetch when window regains focus
    staleTime: 5000, // Consider data stale after 5 seconds
    retry: 3,
  });
}

export function useOnt(id: string) {
  return useQuery({
    queryKey: ["onts", id],
    queryFn: () => ontRepository.getById(id),
    enabled: !!id,
  });
}

export function useCreateOnt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateOntDto) => ontRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["onts"] });
      queryClient.invalidateQueries({ queryKey: ["olts"] });
    },
  });
}

export function useUpdateOnt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateOntDto }) =>
      ontRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["onts"] });
      queryClient.invalidateQueries({ queryKey: ["onts", variables.id] });
    },
  });
}

export function useDeleteOnt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => ontRepository.delete(id),
    onSuccess: (_, id) => {
      queryClient.removeQueries({ queryKey: ["onts", id] });
      queryClient.invalidateQueries({ queryKey: ["onts"] });
      queryClient.invalidateQueries({ queryKey: ["olts"] });
      queryClient.invalidateQueries({ queryKey: ["polling"] });
    },
  });
}
