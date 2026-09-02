import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";

const csRepository = new CsRepository();

const QUICK_REPLY_KEY = ["cs", "quick-replies"];

/** Canned replies a CS can insert instead of retyping a common answer. */
export function useCsQuickReplies() {
  return useQuery({
    queryKey: QUICK_REPLY_KEY,
    queryFn: () => csRepository.getQuickReplies(),
  });
}

/** Writing a template is admin-only on the API; the picker reads them all. */
export function useCreateQuickReply() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { title: string; body: string }) =>
      csRepository.createQuickReply(vars.title, vars.body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUICK_REPLY_KEY });
    },
    onError: reportCsMutationError,
  });
}

export function useUpdateQuickReply() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { id: string; title: string; body: string }) =>
      csRepository.updateQuickReply(vars.id, vars.title, vars.body),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUICK_REPLY_KEY });
    },
    onError: reportCsMutationError,
  });
}

export function useDeleteQuickReply() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (id: string) => csRepository.deleteQuickReply(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: QUICK_REPLY_KEY });
    },
    onError: reportCsMutationError,
  });
}
