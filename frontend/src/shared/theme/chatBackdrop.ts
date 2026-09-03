/**
 * The doodle wallpaper behind a thread.
 *
 * The artwork is the WhatsApp doodle sheet, supplied for this inbox so the CS
 * screen matches what the team already reads on their phones. It is served as
 * a static file rather than inlined: at 213 KB it would sit in the JS bundle
 * and be parsed on every load, where as a file it is fetched once, cached, and
 * gzipped by nginx down to about 85 KB.
 *
 * The tile does not butt seamlessly — doodles are cut at its edge, so a pane
 * wider than 676px shows a faint join. That is how WhatsApp Web itself renders
 * it, and at this contrast it reads as more doodles rather than as a seam.
 */

/** The wallpaper, ready for CSS `background-image`. */
export const chatBackdropImage = 'url("/chat-backdrop.svg")';

/** The colour under the wallpaper — the sheet's own background, so the edge
 * of a tile and the ground behind it are the same black. */
export const chatBackdropColor = "#090909";
