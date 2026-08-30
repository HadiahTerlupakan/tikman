/** The page sizes the ONT table offers below "show everything". */
const FIXED_PAGE_SIZES = [5, 10, 20, 50, 100];

export const DEFAULT_ONT_PAGE_SIZE = 5;

/**
 * The page sizes to offer for a table of `total` rows.
 *
 * The list used to end in the string "all", which Ant Design hands back as a
 * page size: it is not a number, so choosing it produced NaN rows. It was then
 * offered as the row count itself, which worked while the browser held every
 * row already.
 *
 * The database pages the list now, so "everything" would be a request for the
 * whole network — hundreds of thousands of rows on the installations this is
 * built for. Only fixed sizes are offered, and only those a result this large
 * can fill, so the selector never lists a page the data cannot reach.
 */
export function ontPageSizeOptions(total: number): number[] {
  return FIXED_PAGE_SIZES.filter(
    (size) => size < total || size === FIXED_PAGE_SIZES[0],
  );
}
