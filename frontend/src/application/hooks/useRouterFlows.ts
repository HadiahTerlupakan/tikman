import { useQuery } from "@tanstack/react-query";
import { RouterFlowRepository } from "@/infrastructure/repositories";
import type {
  FlowDirection,
  FlowFilter,
  TrafficRange,
} from "@/domain/entities";
import {
  liveRefetchInterval,
  resolveTrafficRange,
  trafficRangeKey,
} from "./trafficRange";

const repository = new RouterFlowRepository();

// flowd writes once a minute; reading faster returns the same rows.
const FLOW_POLL_MS = 60_000;

function flowKey(
  routerId: string,
  read: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return [
    "mikrotik",
    routerId,
    "flows",
    read,
    ...trafficRangeKey(range),
    filter.pop ?? "",
    filter.upstream ?? "",
    filter.direction ?? "",
  ];
}

export function useFlowDestinations(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return useQuery({
    queryKey: flowKey(routerId, "destinations", range, filter),
    queryFn: () =>
      repository.destinations(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

export function useFlowSummary(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
) {
  return useQuery({
    queryKey: flowKey(routerId, "summary", range, filter),
    queryFn: () =>
      repository.summary(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

export function useFlowPops(
  routerId: string,
  range: TrafficRange,
  direction?: FlowDirection,
) {
  const filter = { direction };
  return useQuery({
    queryKey: flowKey(routerId, "pops", range, filter),
    queryFn: () =>
      repository.pops(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
  });
}

/** Disabled past seven days: the server keeps top addresses no longer. */
export function useFlowTopIps(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
  enabled: boolean,
) {
  return useQuery({
    queryKey: flowKey(routerId, "top-ips", range, filter),
    queryFn: () =>
      repository.topIps(routerId, resolveTrafficRange(range), filter),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
    enabled,
  });
}

/** Runs only while a destination is selected. */
export function useFlowPeering(
  routerId: string,
  range: TrafficRange,
  filter: FlowFilter,
  asn: number | null,
) {
  return useQuery({
    queryKey: [...flowKey(routerId, "peering", range, filter), asn ?? ""],
    queryFn: () =>
      repository.peering(
        routerId,
        resolveTrafficRange(range),
        filter,
        asn ?? 0,
      ),
    refetchInterval: liveRefetchInterval(range, FLOW_POLL_MS),
    enabled: asn !== null,
  });
}

export function useFlowStatus(routerId: string) {
  return useQuery({
    queryKey: ["mikrotik", routerId, "flows", "status"],
    queryFn: () => repository.status(routerId),
    refetchInterval: FLOW_POLL_MS,
  });
}
