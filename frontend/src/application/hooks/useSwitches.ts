import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { SwitchRepository } from "@/infrastructure/repositories";
import type { CreateSwitchDto, UpdateSwitchDto } from "@/domain/entities";

const repository = new SwitchRepository();

// Matches the worker's one-minute interfaces tier: polling faster only
// re-reads the same rows.
const POLL_INTERVAL_MS = 60_000;

export function useSwitches() {
  return useQuery({
    queryKey: ["switches"],
    queryFn: () => repository.list(),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useSwitchInterfaces(switchId: string) {
  return useQuery({
    queryKey: ["switches", switchId, "interfaces"],
    queryFn: () => repository.interfaces(switchId),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useSwitchVlans(switchId: string) {
  return useQuery({
    queryKey: ["switches", switchId, "vlans"],
    queryFn: () => repository.vlans(switchId),
  });
}

export function useSwitchNeighbors(switchId: string) {
  return useQuery({
    queryKey: ["switches", switchId, "neighbors"],
    queryFn: () => repository.neighbors(switchId),
  });
}

export function useSwitchBackups(switchId: string) {
  return useQuery({
    queryKey: ["switches", switchId, "backups"],
    queryFn: () => repository.backups(switchId),
  });
}

export function useSwitchBackup(switchId: string, backupId: string | null) {
  return useQuery({
    queryKey: ["switches", switchId, "backups", backupId],
    queryFn: () => repository.backup(switchId, backupId ?? ""),
    enabled: backupId !== null,
  });
}

export function useSwitchBackupDiff(
  switchId: string,
  from: string | null,
  to: string | null,
) {
  return useQuery({
    queryKey: ["switches", switchId, "backups", "diff", from, to],
    queryFn: () => repository.backupDiff(switchId, from ?? "", to ?? ""),
    enabled: from !== null && to !== null,
  });
}

export function useSwitchBackupNow() {
  return useSwitchMutation((id: string) => repository.backupNow(id));
}

export function useTrustSwitchHostKey() {
  return useSwitchMutation(
    ({ id, fingerprint }: { id: string; fingerprint: string }) =>
      repository.trustHostKey(id, fingerprint),
  );
}

function useSwitchMutation<TArgs, TResult>(
  mutationFn: (args: TArgs) => Promise<TResult>,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["switches"] });
    },
  });
}

export function useCreateSwitch() {
  return useSwitchMutation((data: CreateSwitchDto) => repository.create(data));
}

export function useUpdateSwitch() {
  return useSwitchMutation(
    ({ id, data }: { id: string; data: UpdateSwitchDto }) =>
      repository.update(id, data),
  );
}

export function useTestSwitchConnection() {
  return useMutation({
    mutationFn: (data: CreateSwitchDto) => repository.testConnection(data),
  });
}
