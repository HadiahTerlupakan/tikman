/**
 * How many ONTs a page asks for when it needs a whole OLT at once.
 *
 * The ONT list and the graphs page no longer use this: the database filters and
 * pages them, so their request is one page long whatever the network's size.
 * What remains is the OLT configuration page, which reads an OLT's ONTs as a
 * whole to lay out its ports.
 *
 * The API allows up to 5000. This leaves headroom above a full ZTE C320 while
 * keeping the payload bounded.
 */
export const ONT_FETCH_LIMIT = 2000;

/**
 * How long typing has to stop before the list searches.
 *
 * Search runs against the whole ONT table on the server, so a serial typed one
 * character at a time should cost one query rather than twelve.
 */
export const SEARCH_DEBOUNCE_MS = 300;

/**
 * How often the inbox re-asks who is at their desk.
 *
 * Presence has no change event: an agent leaves by their Redis key expiring
 * after sixty seconds, and nothing observes that. Polling faster than this
 * would re-fetch a set that cannot have changed, and the sixty-second TTL
 * already bounds how stale the answer can be.
 */
export const CS_PRESENCE_POLL_MS = 20_000;
