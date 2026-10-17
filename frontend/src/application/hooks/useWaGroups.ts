import { useMemo } from "react";
import {
  useInfiniteQuery,
  useMutation,
  useQuery,
  useQueryClient,
  type QueryClient,
} from "@tanstack/react-query";
import { WaGroupRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";
import { waGroupKeys } from "./waGroupKeys";
import type { WaGroup, WaGroupFilter, WaGroupMessage } from "@/domain/entities";

const waGroupRepository = new WaGroupRepository();

/** How many messages one page of group history holds. */
export const WA_GROUP_HISTORY_PAGE = 50;

/** Every group of every connected number, newest activity first. */
export function useWaGroups(filter?: WaGroupFilter) {
  return useQuery({
    queryKey: waGroupKeys.list(filter),
    queryFn: () => waGroupRepository.list(filter),
  });
}

/** One group with its members. */
export function useWaGroupDetail(groupId?: string) {
  return useQuery({
    queryKey: waGroupKeys.detail(groupId),
    queryFn: () => waGroupRepository.get(groupId as string),
    enabled: !!groupId,
  });
}

/** Reading the first page clears the unread count on the server; this clears
 * it in the cached list too, so the badge drops when the group opens rather
 * than whenever the list next happens to be refetched. */
function markRead(queryClient: QueryClient, groupId: string) {
  queryClient.setQueriesData<WaGroup[]>(
    { queryKey: waGroupKeys.lists() },
    (groups?: WaGroup[]) =>
      groups?.map((g) => (g.id === groupId ? { ...g, unreadCount: 0 } : g)),
  );
}

/** Pages are fetched by offset, so a message arriving between two page loads
 * shifts the window and the same row can land on both pages. */
export function flattenHistory(pages: WaGroupMessage[][]): WaGroupMessage[] {
  const seen = new Set<string>();
  return pages.flat().filter((m) => !seen.has(m.id) && !!seen.add(m.id));
}

/** One group's messages, newest first, with older pages loaded on request. */
export function useWaGroupHistory(groupId?: string) {
  const queryClient = useQueryClient();
  const query = useInfiniteQuery({
    queryKey: waGroupKeys.messages(groupId),
    queryFn: async ({ pageParam }) => {
      const id = groupId as string;
      const page = await waGroupRepository.getHistory(
        id,
        WA_GROUP_HISTORY_PAGE,
        pageParam,
      );
      if (pageParam === 0) markRead(queryClient, id);
      return page;
    },
    initialPageParam: 0,
    getNextPageParam: (last, pages) =>
      last.length < WA_GROUP_HISTORY_PAGE
        ? undefined
        : pages.length * WA_GROUP_HISTORY_PAGE,
    enabled: !!groupId,
  });
  const pages = query.data?.pages;
  const messages = useMemo(() => flattenHistory(pages ?? []), [pages]);
  return { ...query, messages };
}

function useRefetchGroupThread() {
  const queryClient = useQueryClient();
  return (groupId: string) => {
    queryClient.invalidateQueries({ queryKey: waGroupKeys.lists() });
    queryClient.invalidateQueries({ queryKey: waGroupKeys.messages(groupId) });
  };
}

export function useSendWaGroupMessage() {
  const refetch = useRefetchGroupThread();
  return useMutation({
    mutationFn: (vars: {
      groupId: string;
      body: string;
      replyToId?: string;
      mentions?: string[];
    }) =>
      waGroupRepository.sendMessage(
        vars.groupId,
        vars.body,
        vars.replyToId,
        vars.mentions,
      ),
    onSuccess: (_, vars) => refetch(vars.groupId),
    onError: reportCsMutationError,
  });
}

export function useSendWaGroupMedia() {
  const refetch = useRefetchGroupThread();
  return useMutation({
    mutationFn: (vars: {
      groupId: string;
      file: File;
      caption?: string;
      replyToId?: string;
      mentions?: string[];
    }) =>
      waGroupRepository.sendMedia(
        vars.groupId,
        vars.file,
        vars.caption,
        vars.replyToId,
        vars.mentions,
      ),
    onSuccess: (_, vars) => refetch(vars.groupId),
    onError: reportCsMutationError,
  });
}

/** Asks WhatsApp for the group list again. The answer arrives as a group
 * event once it has been read, not in this response. */
export function useRefreshWaGroups() {
  return useMutation({
    mutationFn: (waAccountId?: string) =>
      waGroupRepository.refresh(waAccountId),
    onError: reportCsMutationError,
  });
}
