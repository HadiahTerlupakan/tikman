/**
 * CONTROL_MAX_WIDTH caps a filter control against the screen it is drawn on.
 *
 * These selects carry fixed pixel widths so a desktop toolbar lines up, and a
 * fixed width cannot shrink: measured on a 320px phone, the ONT and Graphs
 * toolbars ran past the edge and dragged the whole page sideways with them.
 *
 * The cap only ever binds on a narrow screen: at any desktop width it is far
 * wider than the widest control here, so the toolbars keep the layout they were
 * designed with. 60 rather than 70 because several of these selects sit beside
 * a label inside the same item, and the label needs room too — measured on the
 * Graphs toolbar, where 70 still left it 42px over the edge.
 */
export const CONTROL_MAX_WIDTH = "60vw";
