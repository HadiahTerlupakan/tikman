import type { Ont, OntStatus } from "@/domain/entities";

export interface OntFilterSelection {
  oltId?: string;
  slot?: number;
  portId?: number;
  searchText?: string;
  status?: OntStatus;
}

// Kept out of the list hook so the rules can be read and tested on their own.
// The card was being collected and then ignored here, which let a port
// selection match that port on every card at once.
export function matchesOntFilters(ont: Ont, filters: OntFilterSelection) {
  if (filters.oltId && ont.oltId !== filters.oltId) {
    return false;
  }

  // An ONT the poll has not placed on a card yet cannot answer the question,
  // so it is left out rather than assumed to be on the selected one.
  if (filters.slot !== undefined && ont.slot !== filters.slot) {
    return false;
  }

  if (filters.portId !== undefined && ont.portId !== filters.portId) {
    return false;
  }

  if (
    filters.searchText &&
    !ont.serialNumber.toLowerCase().includes(filters.searchText.toLowerCase())
  ) {
    return false;
  }

  return !(filters.status && ont.status !== filters.status);
}
