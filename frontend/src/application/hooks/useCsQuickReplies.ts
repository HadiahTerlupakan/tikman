import { useQuery } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";

const csRepository = new CsRepository();

/** Canned replies a CS can insert instead of retyping a common answer. */
export function useCsQuickReplies() {
  return useQuery({
    queryKey: ["cs", "quick-replies"],
    queryFn: () => csRepository.getQuickReplies(),
  });
}
