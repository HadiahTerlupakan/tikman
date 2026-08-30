import { useQueries } from "@tanstack/react-query";
import type { UseQueryResult } from "@tanstack/react-query";
import { OltRepository } from "@/infrastructure/repositories";
import type { DetectedOnu, Olt, UnconfiguredOnu } from "@/domain/entities";

const oltRepository = new OltRepository();

// A scan is a live SNMP walk, but a measured one costs the OLT ~45ms and one to
// three GETBULKs, so the interval is set by how soon an operator wants a newly
// lit ONU to appear, not by load. The OLT's own autofind table takes seconds to
// fill, which is the real floor. Adding OLTs widens the fan-out; each device is
// still scanned once per interval.
const UNCONFIGURED_ONU_POLL_INTERVAL = 15000;

export interface DetectedOnuScan {
  rows: DetectedOnu[];
  /** True only until the first OLT answers, so a slow one cannot hide the rest. */
  isLoading: boolean;
  isFetching: boolean;
  /**
   * OLTs whose scan failed. Their ONUs are missing from rows rather than
   * absent, and an empty table means "none found" only if this is empty too.
   */
  failed: string[];
  rescan: () => void;
}

/**
 * Scans every OLT rather than one the operator picked first. The page used to
 * open on whichever OLT sorted first, which is regularly one with nothing
 * waiting while another has ONUs ready to register — so the operator had to
 * click through sites to find the work.
 */
export function useUnconfiguredOnus(olts: Olt[] | undefined): DetectedOnuScan {
  const list = Array.isArray(olts) ? olts : [];

  return useQueries({
    queries: list.map((olt) => ({
      // The same key the single-OLT scan used, so anything else reading a
      // scan shares this cache rather than walking the OLT a second time.
      queryKey: ["olts", olt.id, "unconfigured-onus"],
      queryFn: () => oltRepository.getUnconfiguredOnus(olt.id),
      refetchInterval: UNCONFIGURED_ONU_POLL_INTERVAL,
    })),
    combine: (results) => mergeScans(list, results),
  });
}

function mergeScans(
  olts: Olt[],
  results: UseQueryResult<UnconfiguredOnu[], Error>[],
): DetectedOnuScan {
  const rows: DetectedOnu[] = [];
  const failed: string[] = [];

  results.forEach((result, index) => {
    const olt = olts[index];
    if (!olt) {
      return;
    }
    if (result.isError) {
      failed.push(olt.name);
      return;
    }
    for (const onu of result.data ?? []) {
      rows.push({ ...onu, oltId: olt.id, oltName: olt.name });
    }
  });

  rows.sort(byLocation);

  return {
    rows,
    isLoading: results.length > 0 && results.every((r) => r.isLoading),
    isFetching: results.some((r) => r.isFetching),
    failed,
    rescan: () => results.forEach((r) => void r.refetch()),
  };
}

// Physical order, so a row keeps its place across polls instead of moving as
// each OLT's scan lands.
function byLocation(a: DetectedOnu, b: DetectedOnu): number {
  return (
    a.oltName.localeCompare(b.oltName) ||
    a.slot - b.slot ||
    a.port - b.port ||
    a.serialNumber.localeCompare(b.serialNumber)
  );
}
