import type { CsConversationFilter } from "@/domain/entities";

/** The four views a CS switches between. They are mutually exclusive on the
 * backend — the first one set wins — which is why this is a segmented control
 * and not a row of checkboxes. */
export type InboxView = "semua" | "milik-saya" | "belum-dibalas" | "selesai";

export const inboxViews: { value: InboxView; label: string }[] = [
  { value: "semua", label: "Semua" },
  { value: "milik-saya", label: "Milik saya" },
  { value: "belum-dibalas", label: "Belum dibalas" },
  { value: "selesai", label: "Selesai" },
];

/** Whether a value off the URL names one of the four views. A ?view= someone
 * typed by hand, or one left over from a renamed view, has to fall back
 * rather than leave the segmented control on a value it cannot show. */
export function isInboxView(value: string | null): value is InboxView {
  return inboxViews.some((view) => view.value === value);
}

/** Turns a view and a search term into the filter the API expects. */
export function filterFor(
  view: InboxView,
  search: string,
): CsConversationFilter {
  const base: CsConversationFilter = search ? { search } : {};
  switch (view) {
    case "milik-saya":
      return { ...base, mine: true };
    case "belum-dibalas":
      return { ...base, awaitingReply: true };
    case "selesai":
      return { ...base, closed: true };
    default:
      return base;
  }
}
