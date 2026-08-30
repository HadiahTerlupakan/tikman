/**
 * How many ONTs a page asks for in one request.
 *
 * The pages that list ONTs load them in one go and filter in the browser, so
 * this bounds what any of them can show. It sat at 500 while a single chassis
 * carried 651, which truncated those pages without telling anyone. The API
 * allows up to 5000; this leaves headroom above a full ZTE C320 while keeping
 * the payload bounded, because the list refreshes every 15 seconds.
 *
 * The overview page does not use this: it reads /dashboard/stats, where the
 * database does the counting and no ONT rows are sent at all.
 */
export const ONT_FETCH_LIMIT = 2000;
