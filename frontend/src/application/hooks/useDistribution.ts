import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DistributionRepository } from "@/infrastructure/repositories";

const repository = new DistributionRepository();

/** Every distribution box on the map (a mapping node of type odp). */
export function useOdps() {
  return useQuery({
    queryKey: ["odps"],
    queryFn: () => repository.listOdps(),
  });
}

/** Who is on which port of one box, asked only when a box is opened. */
export function useOdpSubscribers(odpId?: string) {
  return useQuery({
    queryKey: ["odps", odpId, "subscribers"],
    queryFn: () => repository.subscribersOn(odpId as string),
    enabled: !!odpId,
  });
}

export function useAssignOntToOdp() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { ontId: string; odpId: string; port: number }) =>
      repository.assignOnt(vars.ontId, vars.odpId, vars.port),
    onSuccess: () => {
      // Occupancy moved, so the pin's colour has to move with it.
      queryClient.invalidateQueries({ queryKey: ["odps"] });
      queryClient.invalidateQueries({ queryKey: ["onts"] });
    },
  });
}

export function useUnassignOntFromOdp() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (ontId: string) => repository.unassignOnt(ontId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["odps"] });
      queryClient.invalidateQueries({ queryKey: ["onts"] });
    },
  });
}
