import {
  useMutation,
  useQueries,
  useQuery,
  useQueryClient,
} from "@tanstack/react-query";
import { MikrotikRouterRepository } from "@/infrastructure/repositories";
import type {
  CreateRouterDto,
  RouterInterface,
  RouterTrafficPoint,
  TestRouterConnectionDto,
  TrafficRange,
  UpdateRouterDto,
} from "@/domain/entities";
import {
  liveRefetchInterval,
  resolveTrafficRange,
  trafficRangeKey,
} from "./trafficRange";

const repository = new MikrotikRouterRepository();

// Matches the worker's one-minute interface and BGP tiers: polling faster only
// re-reads the same row.
const POLL_INTERVAL_MS = 60_000;

const routersKey = ["mikrotik", "routers"] as const;

export function useMikrotikRouters() {
  return useQuery({
    queryKey: routersKey,
    queryFn: () => repository.list(),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterInterfaces(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "interfaces"],
    queryFn: () => repository.interfaces(routerId),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterBgpSessions(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "sessions"],
    queryFn: () => repository.sessions(routerId),
    refetchInterval: POLL_INTERVAL_MS,
  });
}

export function useRouterBgpEvents(routerId: string, range: TrafficRange) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "events", ...trafficRangeKey(range)],
    queryFn: () => repository.events(routerId, resolveTrafficRange(range)),
    refetchInterval: liveRefetchInterval(range, POLL_INTERVAL_MS),
  });
}

function trafficQuery(
  routerId: string,
  interfaceId: string,
  range: TrafficRange,
) {
  return {
    queryKey: [
      "mikrotik",
      routerId,
      "traffic",
      interfaceId,
      ...trafficRangeKey(range),
    ],
    queryFn: () =>
      repository.traffic(routerId, interfaceId, resolveTrafficRange(range)),
    refetchInterval: liveRefetchInterval(range, POLL_INTERVAL_MS),
  };
}

export function useInterfaceTraffic(
  routerId: string,
  interfaceId: string,
  range: TrafficRange,
) {
  return useQuery(trafficQuery(routerId, interfaceId, range));
}

/** Traffic of several interfaces at once, keyed by the label each is drawn under. */
export function useInterfacesTraffic(
  routerId: string,
  interfaces: { iface: RouterInterface; label: string }[],
  range: TrafficRange,
): Record<string, RouterTrafficPoint[]> {
  const results = useQueries({
    queries: interfaces.map(({ iface }) =>
      trafficQuery(routerId, iface.id, range),
    ),
  });
  const series: Record<string, RouterTrafficPoint[]> = {};
  interfaces.forEach(({ label }, i) => {
    series[label] = results[i]?.data ?? [];
  });
  return series;
}

function useRouterMutation<TArgs, TResult>(
  mutationFn: (args: TArgs) => Promise<TResult>,
) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["mikrotik"] });
    },
  });
}

export function useCreateMikrotikRouter() {
  return useRouterMutation((data: CreateRouterDto) => repository.create(data));
}

export function useUpdateMikrotikRouter() {
  return useRouterMutation(
    ({ id, data }: { id: string; data: UpdateRouterDto }) =>
      repository.update(id, data),
  );
}

export function useDeleteMikrotikRouter() {
  return useRouterMutation((id: string) => repository.remove(id));
}

export function useRefreshRouter() {
  return useRouterMutation((id: string) => repository.refresh(id));
}

export function useTrustRouterCertificate() {
  return useRouterMutation(
    ({ id, fingerprint }: { id: string; fingerprint: string }) =>
      repository.trustCertificate(id, fingerprint),
  );
}

export function useTestRouterConnection() {
  return useMutation({
    mutationFn: (data: TestRouterConnectionDto) =>
      repository.testConnection(data),
  });
}
