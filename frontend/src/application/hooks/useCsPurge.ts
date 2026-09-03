import { useMutation, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";

const csRepository = new CsRepository();

/**
 * Removing things from the inbox.
 *
 * Every one of these removes the copy held here and nothing else: the message
 * on the customer's phone stays where it is. WhatsApp's own "delete for
 * everyone" only reaches messages the CS sent, and only for a while after
 * sending, so a button offering it would work on some rows and quietly not on
 * others.
 */

/** Invalidates everything a purge can have changed. Thread histories are
 * invalidated by prefix rather than by id: clearing a number or the inbox
 * empties threads this browser may have open in another tab. */
function useInvalidateInbox() {
  const queryClient = useQueryClient();
  return () => {
    queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
    queryClient.invalidateQueries({ queryKey: ["cs", "messages"] });
  };
}

export function useDeleteCsMessage() {
  const invalidate = useInvalidateInbox();
  return useMutation({
    mutationFn: (vars: { messageId: string; conversationId: string }) =>
      csRepository.deleteMessage(vars.messageId),
    onSuccess: invalidate,
    onError: reportCsMutationError,
  });
}

/** Empties one thread. The thread itself stays in the inbox. */
export function useClearCsConversation() {
  const invalidate = useInvalidateInbox();
  return useMutation({
    mutationFn: (conversationId: string) =>
      csRepository.clearConversation(conversationId),
    onSuccess: invalidate,
    onError: reportCsMutationError,
  });
}

/** Empties every thread on one number without removing the number. */
export function useClearWaAccountMessages() {
  const invalidate = useInvalidateInbox();
  return useMutation({
    mutationFn: (id: string) => csRepository.clearWaAccountMessages(id),
    onSuccess: invalidate,
    onError: reportCsMutationError,
  });
}

export function useClearCsInbox() {
  const invalidate = useInvalidateInbox();
  return useMutation({
    mutationFn: () => csRepository.clearInbox(),
    onSuccess: invalidate,
    onError: reportCsMutationError,
  });
}

/**
 * Removes a number along with every thread, message and file on it.
 *
 * The account list is invalidated as well as the inbox: the number is gone
 * from the picker, the badge and the tags on every thread it carried.
 */
export function useDeleteWaAccount() {
  const queryClient = useQueryClient();
  const invalidate = useInvalidateInbox();
  return useMutation({
    mutationFn: (id: string) => csRepository.deleteWaAccount(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "wa-accounts"] });
      invalidate();
    },
    onError: reportCsMutationError,
  });
}
