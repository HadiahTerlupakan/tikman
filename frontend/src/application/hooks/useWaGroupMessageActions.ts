import { useCallback } from "react";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import {
  WaGroupMessageRepository,
  WaGroupRepository,
} from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";
import { waGroupKeys } from "./waGroupKeys";
import type { WaGroupPollRequest } from "@/domain/entities";

const groupRepository = new WaGroupRepository();
const messageRepository = new WaGroupMessageRepository();

interface MessageRef {
  groupId: string;
  messageId: string;
}

/** The queued ones settle over the stream: the group_message event that
 * follows refetches the thread, so nothing is refetched here. */
export function useReactToWaGroupMessage() {
  return useMutation({
    mutationFn: (vars: MessageRef & { emoji: string }) =>
      messageRepository.react(vars.groupId, vars.messageId, vars.emoji),
    onError: reportCsMutationError,
  });
}

export function useEditWaGroupMessage() {
  return useMutation({
    mutationFn: (vars: MessageRef & { body: string }) =>
      messageRepository.edit(vars.groupId, vars.messageId, vars.body),
    onError: reportCsMutationError,
  });
}

export function useRevokeWaGroupMessage() {
  return useMutation({
    mutationFn: (vars: MessageRef) =>
      messageRepository.revoke(vars.groupId, vars.messageId),
    onError: reportCsMutationError,
  });
}

export function useVoteWaGroupPoll() {
  return useMutation({
    mutationFn: (vars: MessageRef & { options: string[] }) =>
      messageRepository.vote(vars.groupId, vars.messageId, vars.options),
    onError: reportCsMutationError,
  });
}

/** A star is TikMan's own record and answers at once, so the thread is
 * refetched here rather than waiting on the stream. */
export function useStarWaGroupMessage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: MessageRef & { starred: boolean }) =>
      messageRepository.setStarred(vars.groupId, vars.messageId, vars.starred),
    onSuccess: (_, vars) =>
      queryClient.invalidateQueries({
        queryKey: waGroupKeys.messages(vars.groupId),
      }),
    onError: reportCsMutationError,
  });
}

export function useForwardWaGroupMessage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: MessageRef & { groupIds: string[] }) =>
      messageRepository.forward(vars.groupId, vars.messageId, vars.groupIds),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: waGroupKeys.lists() });
      for (const id of vars.groupIds) {
        queryClient.invalidateQueries({ queryKey: waGroupKeys.messages(id) });
      }
    },
    onError: reportCsMutationError,
  });
}

/** Answers the inbox conversation the sender's private thread lives in. */
export function useReplyPrivatelyToWaGroupMessage() {
  return useMutation({
    mutationFn: (vars: MessageRef) =>
      messageRepository.replyPrivately(vars.groupId, vars.messageId),
    onError: reportCsMutationError,
  });
}

/** Who received and read one of our messages. */
export function useWaGroupReceipts(groupId?: string, messageId?: string) {
  return useQuery({
    queryKey: waGroupKeys.receipts(groupId ?? "", messageId ?? ""),
    queryFn: () =>
      messageRepository.getReceipts(groupId as string, messageId as string),
    enabled: !!groupId && !!messageId,
  });
}

export function useWaGroupStarred(groupId?: string) {
  return useQuery({
    queryKey: waGroupKeys.starred(groupId ?? ""),
    queryFn: () => groupRepository.getStarred(groupId as string),
    enabled: !!groupId,
  });
}

export function useCreateWaGroupPoll() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { groupId: string; poll: WaGroupPollRequest }) =>
      groupRepository.createPoll(vars.groupId, vars.poll),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: waGroupKeys.lists() });
      queryClient.invalidateQueries({
        queryKey: waGroupKeys.messages(vars.groupId),
      });
    },
    onError: reportCsMutationError,
  });
}

export function usePinWaGroup() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { groupId: string; pinned: boolean }) =>
      groupRepository.setPinned(vars.groupId, vars.pinned),
    onSuccess: () =>
      queryClient.invalidateQueries({ queryKey: waGroupKeys.lists() }),
    onError: reportCsMutationError,
  });
}

/** Asks the phone for older messages; they arrive later over the stream. */
export function useRequestWaGroupHistory() {
  return useMutation({
    mutationFn: (groupId: string) => groupRepository.requestHistory(groupId),
    onError: reportCsMutationError,
  });
}

/** Fire-and-forget, as the inbox's own typing signal is: a lost "sedang
 * mengetik" is not worth an error in front of somebody mid-sentence. */
export function useSetWaGroupTyping() {
  return useCallback((groupId: string, typing: boolean) => {
    void groupRepository.setTyping(groupId, typing).catch(() => {});
  }, []);
}
