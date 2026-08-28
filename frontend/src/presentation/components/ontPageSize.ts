/** The page sizes the ONT table offers below "show everything". */
const FIXED_PAGE_SIZES = [5, 10, 20, 50, 100];

export const DEFAULT_ONT_PAGE_SIZE = 5;

/**
 * The page sizes to offer for a table of `total` rows.
 *
 * The list used to end in the string "all", which Ant Design hands back as a
 * page size: it is not a number, so choosing it produced NaN rows. Showing
 * everything is offered as the row count itself, and only when it is more than
 * the largest fixed size — otherwise the last option would repeat.
 */
export function ontPageSizeOptions(total: number): number[] {
  const sizes = FIXED_PAGE_SIZES.filter(
    (size) => size < total || size === FIXED_PAGE_SIZES[0],
  );
  const largest = sizes[sizes.length - 1] ?? FIXED_PAGE_SIZES[0];

  return total > largest ? [...sizes, total] : sizes;
}
