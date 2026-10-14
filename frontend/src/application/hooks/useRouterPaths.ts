import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { RouterPathRepository } from "@/infrastructure/repositories";

const repository = new RouterPathRepository();

// The dataset refreshes daily; names do not change under a diagram.
const AS_NAMES_STALE_MS = 60 * 60_000;

export function useAsNames(routerId: string, asns: number[]) {
  const sorted = [...new Set(asns)].sort((a, b) => a - b);
  return useQuery({
    queryKey: ["mikrotik", routerId, "as-names", sorted.join(",")],
    queryFn: () => repository.asNames(routerId, sorted),
    enabled: sorted.length > 0,
    staleTime: AS_NAMES_STALE_MS,
  });
}

/** A live lookup; its answer is stored server-side, so the advice is refetched. */
export function useRouteLookup(routerId: string) {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (target: string) => repository.lookup(routerId, target),
    onSuccess: () => {
      queryClient.invalidateQueries({
        queryKey: ["mikrotik", routerId, "flows", "route-advice"],
      });
    },
  });
}
