import { useQuery } from "@tanstack/react-query";
import { OltRepository } from "@/infrastructure/repositories";

const oltRepository = new OltRepository();

// A scan is a live SNMP walk, but a measured one costs the OLT ~45ms and one to
// three GETBULKs, so the interval is set by how soon an operator wants a newly
// lit ONU to appear, not by load. The OLT's own autofind table takes seconds to
// fill, which is the real floor.
const UNCONFIGURED_ONU_POLL_INTERVAL = 15000;

export function useUnconfiguredOnus(oltId?: string) {
  return useQuery({
    queryKey: ["olts", oltId, "unconfigured-onus"],
    queryFn: () => oltRepository.getUnconfiguredOnus(oltId as string),
    enabled: !!oltId,
    refetchInterval: UNCONFIGURED_ONU_POLL_INTERVAL,
  });
}
