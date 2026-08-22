import { useQuery } from "@tanstack/react-query";
import { HealthRepository } from "@/infrastructure/repositories";

const healthRepository = new HealthRepository();

const HEALTH_POLL_INTERVAL = 30000;

export function useHealth() {
  return useQuery({
    queryKey: ["health"],
    queryFn: () => healthRepository.get(),
    refetchInterval: HEALTH_POLL_INTERVAL,
    refetchIntervalInBackground: true,
    // The repository resolves with an "unreachable" result instead of throwing,
    // so a retry would only delay showing the operator that the API is down.
    retry: false,
  });
}
