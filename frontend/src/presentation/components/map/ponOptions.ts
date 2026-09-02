/** One PON port as the OLT reported it. */
export interface TopologyPort {
  portId: number;
}

/** One card as the OLT reported it, with the ports on it. */
export interface TopologyCard {
  slot: number;
  ports?: TopologyPort[];
}

export interface SelectOption {
  value: number;
  label: string;
}

/**
 * The cards this OLT actually has.
 *
 * Read from the topology the poller already stored rather than asked of anyone:
 * a chassis knows its own cards, and typing a slot number that is not there
 * records a distribution box hanging off nothing.
 */
export function cardOptions(
  topology: TopologyCard[] | undefined,
): SelectOption[] {
  return (topology ?? [])
    .map((card) => card.slot)
    .sort((a, b) => a - b)
    .map((slot) => ({ value: slot, label: `Card ${slot}` }));
}

/**
 * The PON ports on one card of this OLT.
 *
 * Empty for a card that is not there, which is what makes changing the card
 * clear the port rather than leave a number from the previous one behind.
 */
export function portOptions(
  topology: TopologyCard[] | undefined,
  slot: number | undefined,
): SelectOption[] {
  if (slot === undefined) {
    return [];
  }
  const card = (topology ?? []).find((entry) => entry.slot === slot);
  return (card?.ports ?? [])
    .map((port) => port.portId)
    .sort((a, b) => a - b)
    .map((portId) => ({ value: portId, label: `PON ${portId}` }));
}
