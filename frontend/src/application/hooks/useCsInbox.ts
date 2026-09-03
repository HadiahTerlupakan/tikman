import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";
import type { CsConversationFilter } from "@/domain/entities";

const csRepository = new CsRepository();

/** The inbox list: everyone's threads, or one of the CS's own views. */
export function useCsConversations(
  filter?: CsConversationFilter,
  options?: { enabled?: boolean },
) {
  return useQuery({
    queryKey: ["cs", "conversations", filter],
    queryFn: () => csRepository.getConversations(filter),
    enabled: options?.enabled,
  });
}

/** One thread's messages, newest first. */
export function useCsHistory(conversationId?: string) {
  return useQuery({
    queryKey: ["cs", "messages", conversationId],
    queryFn: () => csRepository.getHistory(conversationId as string),
    enabled: !!conversationId,
  });
}

export function useSendCsMessage() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: {
      conversationId: string;
      body: string;
      replyToId?: string;
    }) =>
      csRepository.sendMessage(vars.conversationId, vars.body, vars.replyToId),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
      queryClient.invalidateQueries({
        queryKey: ["cs", "messages", vars.conversationId],
      });
    },
    onError: reportCsMutationError,
  });
}

/** Sends a photo or a document on a thread. The caption travels with it, the
 * way WhatsApp itself carries one, rather than as a second message. */
export function useSendCsMedia() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: {
      conversationId: string;
      file: File;
      caption?: string;
      replyToId?: string;
    }) =>
      csRepository.sendMedia(
        vars.conversationId,
        vars.file,
        vars.caption,
        vars.replyToId,
      ),
    onSuccess: (_, vars) => {
      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
      queryClient.invalidateQueries({
        queryKey: ["cs", "messages", vars.conversationId],
      });
    },
    onError: reportCsMutationError,
  });
}

/** Adds a WhatsApp number for the team to answer from. It is only the row —
 * pairing it is a separate, deliberate step. */
export function useCreateWaAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (label: string) => csRepository.createWaAccount(label),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "wa-accounts"] });
    },
    onError: reportCsMutationError,
  });
}

/** Hands a thread to one CS, including taking over one someone else holds. */
export function useAssignConversation() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { conversationId: string; userId: string }) =>
      csRepository.assign(vars.conversationId, vars.userId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
    },
    onError: reportCsMutationError,
  });
}

/**
 * Closes a thread. There is no reopen here — the backend only accepts
 * "closed": a thread reopens on its own when the customer writes again, or a
 * CS takes it back with useAssignConversation.
 */
export function useSetConversationStatus() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { conversationId: string; status: "closed" }) =>
      csRepository.setStatus(vars.conversationId, vars.status),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
    },
    onError: reportCsMutationError,
  });
}

/**
 * Ties a thread to a subscriber's ONT, or unties it when ontId is null.
 * The result also says whether the customer's number reached the ONT row —
 * it does not when that phone is already recorded on a different ONT.
 */
export function useLinkConversationOnt() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { conversationId: string; ontId: string | null }) =>
      csRepository.linkOnt(vars.conversationId, vars.ontId),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
    },
    onError: reportCsMutationError,
  });
}
