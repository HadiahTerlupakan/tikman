import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { OltRepository } from "@/infrastructure/repositories";
import type { CreateOltDto, UpdateOltDto } from "@/domain/entities";

const oltRepository = new OltRepository();

const OLT_LIST_POLL_INTERVAL = 60000;
const OLT_STATS_POLL_INTERVAL = 60000;
const OLT_DISCOVERY_POLL_INTERVAL = 5000; // Refresh quickly while a new OLT is being discovered.

export function useOlts(siteId?: string) {
  return useQuery({
    queryKey: siteId ? ["olts", "site", siteId] : ["olts"],
    queryFn: () =>
      siteId ? oltRepository.getBySite(siteId) : oltRepository.getAll(),
    refetchInterval: OLT_LIST_POLL_INTERVAL,
    refetchIntervalInBackground: true,
  });
}

export function useOlt(id: string) {
  return useQuery({
    queryKey: ["olts", id],
    queryFn: () => oltRepository.getById(id),
    enabled: !!id,
  });
}

export function useOltStats(id: string) {
  return useQuery({
    queryKey: ["olts", id, "stats"],
    queryFn: () => oltRepository.getStats(id),
    enabled: !!id,
    refetchInterval: (query) => {
      const stats = query.state.data;
      if (!stats) return OLT_DISCOVERY_POLL_INTERVAL;
      // The bar only moves while a walk is registering ONTs. Keying this on
      // totalOnts alone dropped to the slow interval as soon as the first
      // instalment landed, so the rest of the discovery advanced once a minute.
      const discovering =
        stats.phase === "discovering" || stats.phase === "polling";
      return discovering || stats.totalOnts === 0
        ? OLT_DISCOVERY_POLL_INTERVAL
        : OLT_STATS_POLL_INTERVAL;
    },
    refetchIntervalInBackground: true,
  });
}

export function useCreateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateOltDto) => oltRepository.create(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["olts"] });
      queryClient.invalidateQueries({ queryKey: ["sites"] });
    },
  });
}

export function useUpdateOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateOltDto }) =>
      oltRepository.update(id, data),
    onSuccess: (_, variables) => {
      queryClient.invalidateQueries({ queryKey: ["olts"] });
      queryClient.invalidateQueries({ queryKey: ["olts", variables.id] });
      queryClient.invalidateQueries({ queryKey: ["sites"] });
    },
  });
}

export function useDeleteOlt() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => oltRepository.delete(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["olts"] });
      queryClient.invalidateQueries({ queryKey: ["sites"] });
    },
  });
}
