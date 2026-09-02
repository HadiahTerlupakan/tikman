import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { DistributionRepository } from "@/infrastructure/repositories";
import type { CreateOdcDto, CreateOdpDto, RoutePoint } from "@/domain/entities";

const repository = new DistributionRepository();

/** Every cabinet, with how many feeds and boxes hang off it. */
export function useOdcs() {
  return useQuery({
    queryKey: ["odcs"],
    queryFn: () => repository.listOdcs(),
  });
}

/** Every distribution box, with the ports already taken on it. */
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

export function useCreateOdc() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateOdcDto) => repository.createOdc(data),
    onSuccess: () => queryClient.invalidateQueries({ queryKey: ["odcs"] }),
  });
}

export function useCreateOdp() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (data: CreateOdpDto) => repository.createOdp(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["odps"] });
      // A new box under a cabinet changes that cabinet's fan-out count.
      queryClient.invalidateQueries({ queryKey: ["odcs"] });
    },
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

/** Every feeder, so the map can draw them all in one pass. */
export function useOdcFeeds() {
  return useQuery({
    queryKey: ["odc-feeds"],
    queryFn: () => repository.listOdcFeeds(),
  });
}

/**
 * Records the path a cable takes. An empty path hands it back to the straight
 * line, which is what choosing "automatic" means.
 */
export function useSetCableRoute() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: {
      kind: "feeder" | "distribution";
      id: string;
      route: RoutePoint[];
    }) =>
      vars.kind === "feeder"
        ? repository.setOdcFeedRoute(vars.id, vars.route)
        : repository.setOdpRoute(vars.id, vars.route),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["odps"] });
      queryClient.invalidateQueries({ queryKey: ["odc-feeds"] });
    },
  });
}
