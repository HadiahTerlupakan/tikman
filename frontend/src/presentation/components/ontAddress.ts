// Every OLT here is a single shelf, and every command TikMan sends addresses
// rack 1. Showing it in the label keeps the address readable as rack/card/pon
// without offering a choice that cannot be made.
const RACK = 1;

export function ontPositionLabel(slot: number, port?: number) {
  return port === undefined ? `${RACK}/${slot}` : `${RACK}/${slot}/${port}`;
}

/**
 * The address the OLT's own CLI uses for one ONU: rack/card/pon:onu. Showing
 * the PON and ONU numbers on their own left out the card, so two ONUs on
 * different cards read identically.
 */
export function ontAddressLabel(
  slot: number | undefined,
  port: number,
  ontId: number,
): string {
  if (slot === undefined) return `PON ${port} · ONU ${ontId}`;
  return `${ontPositionLabel(slot, port)}:${ontId}`;
}
