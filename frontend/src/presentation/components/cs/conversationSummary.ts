import type { CsLastMessage } from "@/domain/entities";

// Media arrives with an empty body, so a preview built from the body alone
// would show a blank line where a photo is the whole message.
const kindLabels: Record<CsLastMessage["kind"], string> = {
  text: "",
  image: "Foto",
  document: "Dokumen",
  audio: "Pesan suara",
  video: "Video",
};

/** The one-line summary under a customer's name, in the inbox list and in the
 * navbar bell's dropdown — shared so the two cannot describe the same thread
 * differently. */
export function preview(last?: CsLastMessage): string {
  if (!last) return "Belum ada pesan";
  const label = kindLabels[last.kind];
  const body = last.body.trim();
  if (label) return body ? `${label} · ${body}` : label;
  return body || "Pesan kosong";
}

// Today shows a clock, anything older shows a date — the same shorthand every
// messaging app uses, because a bare timestamp on a week-old thread is noise.
export function shortTime(iso: string): string {
  const at = new Date(iso);
  const sameDay = new Date().toDateString() === at.toDateString();
  return at.toLocaleString("id-ID", {
    ...(sameDay
      ? { hour: "2-digit", minute: "2-digit" }
      : { day: "2-digit", month: "short" }),
  });
}
