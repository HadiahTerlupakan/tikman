/**
 * The doodle wallpaper behind a thread.
 *
 * It is drawn here rather than shipped as an image: a tiling SVG is a couple
 * of kilobytes against a wallpaper's hundreds, stays sharp on any display, and
 * takes its colours from the theme instead of baking one background in. The
 * drawings are our own — the point is the familiar texture of a chat, not
 * somebody else's artwork.
 */

const ink = "%2326262c";
const tile = 260;

/** Each doodle is drawn around the origin so it can be placed at any angle.
 * Laid out on a grid instead, the wallpaper reads as a sheet of icons — the
 * scatter is what makes it a texture rather than a table. */
const shapes = {
  bubble:
    "M-13-10h26a3 3 0 0 1 3 3v12a3 3 0 0 1-3 3H-3l-6 5v-5h-4a3 3 0 0 1-3-3v-12a3 3 0 0 1 3-3z",
  heart:
    "M0 12c-9-6-13-10-13-16a6.5 6.5 0 0 1 13-3 6.5 6.5 0 0 1 13 3c0 6-4 10-13 16z",
  star: "M0-13l4 8.5 9.5 1-7 6.5 2 9.5-8.5-5-8.5 5 2-9.5-7-6.5 9.5-1z",
  smiley:
    "M12 0a12 12 0 1 1-24 0 12 12 0 0 1 24 0M-5-4v2M5-4v2M-6 4c3 4 9 4 12 0",
  note: "M-6 10V-8l12-3v18M-10 11a4 4 0 1 0 8 0 4 4 0 0 0-8 0M2 8a4 4 0 1 0 8 0 4 4 0 0 0-8 0",
  camera:
    "M-14-6h6l3-4h10l3 4h6a2 2 0 0 1 2 2v14a2 2 0 0 1-2 2h-28a2 2 0 0 1-2-2V-4a2 2 0 0 1 2-2M0 10a7 7 0 1 0 0-14 7 7 0 0 0 0 14",
  cup: "M-10-8h20v12a10 10 0 0 1-20 0zM10-4h5a4 4 0 0 1 0 8h-5M-14 10h28",
  cloud: "M-14 8a7 7 0 0 1 1-14 10 10 0 0 1 19-3 7 7 0 0 1 2 17z",
  envelope: "M-15-9h30v18h-30zM-15-9l15 11 15-11",
  sun: "M0-7a7 7 0 1 0 0 14 7 7 0 0 0 0-14M0-16v4M0 12v4M-16 0h4M12 0h4M-11-11l3 3M8 8l3 3M11-11l-3 3M-8 8l-3 3",
  paw: "M-11-4a4 4 0 1 0 0-8 4 4 0 0 0 0 8M0-6a4 4 0 1 0 0-8 4 4 0 0 0 0 8M11-4a4 4 0 1 0 0-8 4 4 0 0 0 0 8M-9 6a9 9 0 0 1 18 0c0 6-4 8-9 8s-9-2-9-8z",
  umbrella: "M-14 0a14 14 0 0 1 28 0zM0 0v13a5 5 0 0 1-9 2",
  clock: "M0-13a13 13 0 1 0 0 26 13 13 0 0 0 0-26M0-8v9l6 3",
  bolt: "M2-14l-11 18h9l-3 14 12-19h-9z",
  gift: "M-13-4h26v16h-26zM-15-10h30v6h-30zM0-10v22",
  quotes: "M-8-4c-4 2-6 5-6 9h6zM6-4c-4 2-6 5-6 9h6z",
  dot: "M-1.7 0a1.7 1.7 0 1 0 3.4 0 1.7 1.7 0 1 0-3.4 0",
} as const;

/** [shape, x, y, rotation, scale]. Nothing sits within its own radius of an
 * edge: the tile repeats rather than mirrors, so a doodle crossing the edge is
 * cut in half and the seam becomes the pattern's most visible feature. */
const placements: [keyof typeof shapes, number, number, number, number][] = [
  ["bubble", 30, 36, -14, 0.85],
  ["star", 96, 20, 12, 0.75],
  ["heart", 150, 54, -18, 0.7],
  ["smiley", 222, 30, 8, 0.75],
  ["camera", 36, 102, 15, 0.7],
  ["cup", 104, 86, -9, 0.7],
  ["cloud", 196, 98, 6, 0.8],
  ["note", 244, 76, -16, 0.75],
  ["sun", 26, 166, 10, 0.7],
  ["paw", 92, 148, -13, 0.7],
  ["umbrella", 166, 140, 17, 0.75],
  ["envelope", 232, 158, -7, 0.75],
  ["bolt", 56, 220, 14, 0.8],
  ["clock", 124, 206, -11, 0.7],
  ["gift", 190, 230, 9, 0.7],
  ["quotes", 244, 214, -15, 0.8],
  ["star", 186, 16, 14, 0.5],
  ["quotes", 66, 128, 20, 0.5],
  ["bubble", 142, 178, 16, 0.55],
  ["sun", 200, 190, -14, 0.5],
  ["heart", 10, 240, -12, 0.55],
  ["dot", 68, 62, 0, 1],
  ["dot", 132, 124, 0, 1],
  ["dot", 198, 44, 0, 1],
  ["dot", 12, 132, 0, 1],
  ["dot", 250, 122, 0, 1],
  ["dot", 78, 186, 0, 1],
  ["dot", 150, 250, 0, 1],
  ["dot", 208, 176, 0, 1],
];

const doodles = placements
  .map(
    ([shape, x, y, rotation, scale]) =>
      `<path transform='translate(${x} ${y}) rotate(${rotation}) scale(${scale})' d='${shapes[shape]}'/>`,
  )
  .join("");

const svg =
  `<svg xmlns='http://www.w3.org/2000/svg' width='${tile}' height='${tile}' viewBox='0 0 ${tile} ${tile}'>` +
  `<g fill='none' stroke='${ink}' stroke-width='1.7' stroke-linecap='round' stroke-linejoin='round'>${doodles}</g>` +
  `</svg>`;

/** The wallpaper, ready for CSS `background-image`. The angle brackets are
 * escaped rather than left raw: browsers mostly tolerate them inside a data
 * URI, and the ones that do not fail by drawing nothing at all. */
export const chatBackdropImage = `url("data:image/svg+xml,${svg
  .replace(/</g, "%3C")
  .replace(/>/g, "%3E")}")`;

/** The colour under the wallpaper. A shade below the panel, so the thread
 * reads as the surface bubbles sit on rather than more chrome. */
export const chatBackdropColor = "#101012";
