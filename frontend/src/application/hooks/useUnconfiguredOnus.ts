import { useQuery } from "@tanstack/react-query";
import { OltRepository } from "@/infrastructure/repositories";

const oltRepository = new OltRepository();

// Each scan is a live SNMP walk against the OLT, so this polls far less often
// than the cached list endpoints.
const UNCONFIGURED_ONU_POLL_INTERVAL = 120000;

export function useUnconfiguredOnus(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "unconfigured-onus"],
    queryFn: () => oltRepository.getUnconfiguredOnus(oltId as string),
    enabled: !!oltId,
    refetchInterval: UNCONFIGURED_ONU_POLL_INTERVAL,
  });
}
