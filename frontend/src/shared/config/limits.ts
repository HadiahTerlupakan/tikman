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
