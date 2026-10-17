/** Name colours for the people in a group, light enough to read on the dark
 * bubble and far enough apart that neighbours in a busy thread differ. */
const SENDER_COLORS = [
  "#fbbf24",
  "#60a5fa",
  "#f472b6",
  "#34d399",
  "#a78bfa",
  "#fb923c",
  "#22d3ee",
  "#f87171",
  "#a3e635",
  "#e879f9",
  "#2dd4bf",
  "#facc15",
];

/**
 * The colour a member's name takes, the same one every time and on every
 * screen: it is derived from the jid alone, as WhatsApp keeps a person's
 * colour from one day to the next. A hash rather than an index into the
 * member list, which would recolour everyone whenever somebody joined.
 */
export function senderColor(jid: string): string {
  let hash = 0;
  for (let i = 0; i < jid.length; i++) {
    hash = (hash * 31 + jid.charCodeAt(i)) | 0;
  }
  return SENDER_COLORS[Math.abs(hash) % SENDER_COLORS.length];
}
