import type { QueryClient } from "@tanstack/react-query";
import type { WaGroupFilter } from "@/domain/entities";

/**
 * Every group query lives under ["cs", "groups"], a branch the inbox's own
 * keys (["cs", "conversations"], ["cs", "messages", id]) never share, so
 * refreshing one side cannot reload the other.
 */
export const waGroupKeys = {
  all: ["cs", "groups"] as const,
  lists: () => ["cs", "groups", "list"] as const,
  list: (filter?: WaGroupFilter) => ["cs", "groups", "list", filter] as const,
  details: () => ["cs", "groups", "detail"] as const,
  detail: (id?: string) => ["cs", "groups", "detail", id] as const,
  messages: (id?: string) => ["cs", "groups", "messages", id] as const,
  action: (id: string) => ["cs", "groups", "actions", id] as const,
};

const GROUP_EVENT_TYPES = new Set(["group_message", "group", "group_action"]);

/** The stream fields a group event carries. */
export interface GroupStreamEvent {
  type: string;
  group_id?: string;
  action_id?: string;
}

export function isGroupEvent(type: string): boolean {
  return GROUP_EVENT_TYPES.has(type);
}

/**
 * Refetches what one group event changed. The list is always among it: a new
 * message moves a group to the top, and an action can rename or remove one.
 */
export function invalidateForGroupEvent(
  queryClient: QueryClient,
  event: GroupStreamEvent,
) {
  queryClient.invalidateQueries({ queryKey: waGroupKeys.lists() });

  if (event.type === "group_message" && event.group_id) {
    queryClient.invalidateQueries({
      queryKey: waGroupKeys.messages(event.group_id),
    });
    return;
  }

  // A group event with no group named covers the whole list, members and all.
  if (event.type === "group") {
    queryClient.invalidateQueries({
      queryKey: event.group_id
        ? waGroupKeys.detail(event.group_id)
        : waGroupKeys.details(),
    });
    return;
  }

  if (event.type === "group_action") {
    if (event.action_id) {
      queryClient.invalidateQueries({
        queryKey: waGroupKeys.action(event.action_id),
      });
    }
    if (event.group_id) {
      queryClient.invalidateQueries({
        queryKey: waGroupKeys.detail(event.group_id),
      });
    }
  }
}
