import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { OntRepository } from "@/infrastructure/repositories";
import type { CreateOntDto, UpdateOntDto, OntStatus } from "@/domain/entities";

const ontRepository = new OntRepository();

export function useOnts(params?: {
  oltId?: string;
  status?: OntStatus;
  slot?: number;
  portId?: number;
  search?: string;
  startTime?: string;
  endTime?: string;
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

// The commands a removal would send. Fetched only when the operator opens the
// dialog, because it reads the ONT's position rather than the OLT itself.
export function useOntRemovalPreview(ontId?: string) {
  return useQuery({
    queryKey: ["onts", ontId, "removal-preview"],
    queryFn: () => ontRepository.previewRemoval(ontId as string),
    enabled: !!ontId,
    staleTime: Infinity,
  });
}

export function useDeleteOnt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({
      id,
      removeFromOlt,
    }: {
      id: string;
      removeFromOlt: boolean;
    }) => ontRepository.delete(id, removeFromOlt),
    onSuccess: (_, { id }) => {
      queryClient.removeQueries({ queryKey: ["onts", id] });
      queryClient.invalidateQueries({ queryKey: ["onts"] });
      queryClient.invalidateQueries({ queryKey: ["olts"] });
      queryClient.invalidateQueries({ queryKey: ["polling"] });
    },
  });
}

// The ONU's service as the last OLT poll read it, used to open the configure
// form on what is actually running rather than on blank fields.
export function useOntServiceConfig(ontId?: string) {
  return useQuery({
    queryKey: ["onts", ontId, "service-config"],
    queryFn: () => ontRepository.getServiceConfig(ontId as string),
    enabled: !!ontId,
  });
}

// Subscribers ranked by churn. The ONT list answers "is this one up", which an
// ONU that drops and returns every few seconds passes every time it is asked;
// this answers which ones keep failing whatever they read at this instant.
export function useTroubledOnts(
  hours: number,
  oltId?: string,
  status?: string,
) {
  return useQuery({
    queryKey: ["onts", "troubled", hours, oltId ?? "all", status ?? "all"],
    queryFn: () => ontRepository.getTroubled(hours, oltId, status),
    refetchInterval: 60000,
    staleTime: 30000,
  });
}

/**
 * The cards and PON ports one OLT actually has, from the topology the poller
 * already stored.
 *
 * The cached endpoint, not a fresh discovery: filling in a form must never
 * reach out and talk to a live chassis.
 */
export function useOltTopology(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "topology"],
    queryFn: () => ontRepository.getTopology(oltId as string),
    enabled: !!oltId,
  });
}
