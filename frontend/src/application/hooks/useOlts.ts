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

// The VLAN list is refreshed by the discovery poll and stored with the OLT, so
// reading it here costs no SNMP traffic and does not need its own polling.
export function useOltVlans(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "vlans"],
    queryFn: () => oltRepository.getVlans(oltId as string),
    enabled: !!oltId,
  });
}

// T-CONT profile names, read from the OLT's CLI by the discovery poll and
// stored with the OLT, so the form needs no session of its own.
export function useOltTcontProfiles(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "tcont-profiles"],
    queryFn: () => oltRepository.getTcontProfiles(oltId as string),
    enabled: !!oltId,
  });
}

// VLAN profile names in use on the OLT's ONUs, recovered by the poll because
// the CLI has no command that lists them.
export function useOltVlanProfiles(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "vlan-profiles"],
    queryFn: () => oltRepository.getVlanProfiles(oltId as string),
    enabled: !!oltId,
  });
}

// The ONU types the OLT will accept in a registration command. These are not
// the model strings ONUs report over OMCI, which the OLT rejects.
export function useOltOnuTypes(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "onu-types"],
    queryFn: () => oltRepository.getOnuTypes(oltId as string),
    enabled: !!oltId,
  });
}

// The chassis summary and port inventory the discovery poll reads over SNMP.
// Cached with the OLT, so the configuration page costs the device nothing to
// open; it refreshes on the same cadence as the OLT list.
export function useOltSystem(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "system"],
    queryFn: () => oltRepository.getSystem(oltId as string),
    enabled: !!oltId,
    refetchInterval: OLT_LIST_POLL_INTERVAL,
  });
}

// Re-reads the OLT over SNMP now instead of waiting for the discovery poll.
export function useRefreshOltSystem(oltId: string) {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: () => oltRepository.refreshSystem(oltId),
    onSuccess: (snapshot) => {
      queryClient.setQueryData(["olts", oltId, "system"], snapshot);
      queryClient.invalidateQueries({ queryKey: ["olts", oltId, "vlans"] });
    },
  });
}

// Brings an OLT's inventory pass forward. Discovery is on a six-hour schedule,
// which suits ONUs that appear a few times a day but not a technician standing
// at the cabinet having just installed one.
export function useDiscoverOltNow() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (oltId: string) => oltRepository.discoverNow(oltId),
    onSuccess: () => {
      // The worker publishes discovery progress onto the OLT row, so the list
      // is what shows the pass starting.
      queryClient.invalidateQueries({ queryKey: ["olts"] });
    },
  });
}

// Summed traffic under an OLT, or under one PON port. It reads the same tiered
// stores a per-ONT graph does, so a wide window is answerable.
export function useOltAggregateTraffic(
  oltId: string | undefined,
  period: string,
  position?: { slot?: number; port?: number },
) {
  return useQuery({
    queryKey: [
      "olts",
      oltId,
      "traffic",
      period,
      position?.slot,
      position?.port,
    ],
    queryFn: () =>
      oltRepository.getAggregateTraffic(oltId as string, period, position),
    enabled: !!oltId,
    refetchInterval: OLT_LIST_POLL_INTERVAL,
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
