import { useEffect, useRef, useState } from "react";
import { useMutation, useQueries } from "@tanstack/react-query";
import { WaGroupRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";
import { waGroupKeys } from "./waGroupKeys";
import type { WaGroupAction, WaGroupActionRequest } from "@/domain/entities";

const waGroupRepository = new WaGroupRepository();

/** How often a queued action is re-read when no stream event has settled it.
 * The group_action event is what normally does; this only covers a stream
 * that dropped while the action ran, which would leave it queued forever. */
const ACTION_FALLBACK_POLL_MS = 15_000;

/** Creates a group on one of our numbers. Admin only on the server. */
export function useCreateWaGroup() {
  return useMutation({
    mutationFn: (vars: {
      waAccountId: string;
      name: string;
      participants: string[];
    }) =>
      waGroupRepository.create(vars.waAccountId, vars.name, vars.participants),
    onError: reportCsMutationError,
  });
}

/** Queues one management action on a group. Admin only on the server. */
export function useWaGroupAction() {
  return useMutation({
    mutationFn: (vars: { groupId: string; request: WaGroupActionRequest }) =>
      waGroupRepository.act(vars.groupId, vars.request),
    onError: reportCsMutationError,
  });
}

/**
 * Follows queued actions until each one is done or failed, and hands every
 * settled action to onSettled exactly once.
 *
 * The stream's group_action event invalidates the action's query (see
 * invalidateForGroupEvent), which is what refetches it here.
 */
export function useWaGroupActionTracker(
  onSettled: (action: WaGroupAction) => void,
) {
  const [pending, setPending] = useState<string[]>([]);
  const reported = useRef(new Set<string>());
  const settle = useRef(onSettled);
  settle.current = onSettled;

  const results = useQueries({
    queries: pending.map((id) => ({
      queryKey: waGroupKeys.action(id),
      queryFn: () => waGroupRepository.getAction(id),
      refetchInterval: (query: { state: { data?: WaGroupAction } }) =>
        query.state.data?.status === "queued" ? ACTION_FALLBACK_POLL_MS : false,
    })),
  });

  const settled = results
    .map((r) => r.data)
    .filter((a): a is WaGroupAction => !!a && a.status !== "queued");
  const settledKey = settled.map((a) => a.id).join(",");

  useEffect(() => {
    if (!settledKey) return;
    for (const action of settled) {
      if (reported.current.has(action.id)) continue;
      reported.current.add(action.id);
      settle.current(action);
    }
    setPending((ids) => ids.filter((id) => !reported.current.has(id)));
    // settled is derived from results on every render; its ids are the key.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [settledKey]);

  return (action: WaGroupAction) => {
    if (action.status !== "queued") {
      reported.current.add(action.id);
      settle.current(action);
      return;
    }
    setPending((ids) => (ids.includes(action.id) ? ids : [...ids, action.id]));
  };
}
