import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import { reportCsMutationError } from "./csMutationError";

const csRepository = new CsRepository();

/**
 * The WhatsApp numbers the team answers from. Reading this list is open to
 * everyone the CS inbox is open to — a connection that has been down for
 * hours produces no live event to tell a non-admin about it, so the initial
 * fetch is what makes the badge honest for them too. Only connecting or
 * disconnecting a number stays admin-only (see useConnectWaAccount).
 */
export function useWaAccounts() {
  return useQuery({
    queryKey: ["cs", "wa-accounts"],
    queryFn: () => csRepository.listWaAccounts(),
  });
}

/** Starts pairing a number. The linking code itself arrives later over the
 * SSE stream (see useCsStream), not in this response. */
export function useConnectWaAccount() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { id: string; phone: string }) =>
      csRepository.connectWaAccount(vars.id, vars.phone),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["cs", "wa-accounts"] });
    },
    onError: reportCsMutationError,
  });
}
