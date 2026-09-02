import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";

const csRepository = new CsRepository();

/**
 * The WhatsApp numbers the team answers from. Listing them is admin-only on
 * the backend, so callers gate this on the current user's role — a non-admin
 * calling it just gets a 403, not "no accounts".
 */
export function useWaAccounts(enabled: boolean) {
  return useQuery({
    queryKey: ["cs", "wa-accounts"],
    queryFn: () => csRepository.listWaAccounts(),
    enabled,
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
  });
}
