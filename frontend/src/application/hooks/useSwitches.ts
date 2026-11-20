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
