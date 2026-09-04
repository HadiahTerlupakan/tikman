import type { Screens } from "@/presentation/components/layout/layoutPadding";

/** One of the inbox's three surfaces, when only one of them fits. */
export type InboxPane = "list" | "thread" | "customer";

export interface InboxLayout {
  /** True when the three panes sit side by side and nothing navigates. */
  columns: boolean;
  /** The pane to show when they do not. Meaningless while `columns` is true. */
  pane: InboxPane;
}

/**
 * inboxLayout decides whether the inbox shows its three panes at once or one
 * at a time, and which one.
 *
 * The three columns need roughly a thousand pixels — the list alone is fixed
 * at 340 — so on a phone they take turns instead, the way a chat app does:
 * the list, then the conversation, then the customer behind its header.
 *
 * Which pane is showing lives in the URL rather than in state, so the phone's
 * own back button walks back through them, and a link still opens what it
 * names. Keyed on `xs` for the reason layoutPadding is: Ant hands back an
 * empty object before it has measured anything, and reading that as a phone
 * would flash the single-pane layout on every desktop load.
 */
export function inboxLayout(
  screens: Screens,
  selectedId: string | undefined,
  panel: string | null,
): InboxLayout {
  if (!screens.xs) {
    return { columns: true, pane: "list" };
  }
  if (!selectedId) {
    // Including when the URL asks for the customer pane: there is no customer
    // without a conversation, and an empty thread would be a worse answer.
    return { columns: false, pane: "list" };
  }
  return { columns: false, pane: panel === "customer" ? "customer" : "thread" };
}
