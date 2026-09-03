/**
 * The doodle wallpaper behind a thread.
 *
 * The artwork is the WhatsApp doodle sheet, supplied for this inbox so the CS
 * screen matches what the team already reads on their phones. It is served as
 * a static file rather than inlined: at 213 KB it would sit in the JS bundle
 * and be parsed on every load, where as a file it is fetched once, cached, and
 * gzipped by nginx down to about 85 KB.
 *
 * The sheet does not tile — its doodles are cut at the edge — so the file
 * arranges it as a mirrored 2x2 block. Opposite edges of that block are then
 * the same column of artwork and repeating it shows no join. Mirroring costs a
 * reflection axis every half tile; against a hard seam every tile width, which
 * is what the plain sheet gives once it is scaled down, that is the better
 * trade. The block is drawn from a single copy in <defs>, so it is four times
 * the area at the same file size.
 */

/** The wallpaper, ready for CSS `background-image`. */
export const chatBackdropImage = 'url("/chat-backdrop.svg")';

/**
 * Half the artwork's natural size. At 1:1 the doodles crowd the bubbles and
 * read as illustrations rather than as texture; halved, they are dense enough
 * to be a surface. It also puts the block's own edge past the width of any
 * pane the inbox is likely to get, so the repeat is rarely on screen at all.
 */
export const chatBackdropSize = "676px 1200px";

/** The colour under the wallpaper — the sheet's own background, so the edge
 * of a tile and the ground behind it are the same black. */
export const chatBackdropColor = "#090909";
