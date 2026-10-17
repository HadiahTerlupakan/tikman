import type { WaGroup, WaGroupMessage } from "@/domain/entities";

/** WhatsApp's own limit on editing a sent message; the server answers 409
 * EDIT_WINDOW past it. */
export const EDIT_WINDOW_MS = 15 * 60 * 1000;

export type GroupMessageAction =
  | "reply"
  | "react"
  | "forward"
  | "star"
  | "info"
  | "edit"
  | "revoke"
  | "replyPrivate";

export interface GroupMessageActionContext {
  group: WaGroup;
  /** The viewer is a TikMan admin. */
  isAdmin: boolean;
  /** The viewer's own id: only a message's writer may edit it. */
  userId: string;
  /** The composer is open: not left, not admin-only for a non-admin number. */
  canSend: boolean;
  now: number;
}

/** Kinds that carry nothing a copy could re-send. */
const UNFORWARDABLE = new Set(["system", "poll"]);

/**
 * What the bubble menu offers on one message, in menu order. Each rule is the
 * server's own, so nothing is offered only to be refused: an edit only on
 * text the viewer wrote, inside the window; a delete for everyone on what the
 * viewer wrote, on anything the number sent for an admin, and on a member's
 * message when both the viewer and our number are admins.
 */
export function groupMessageActions(
  message: WaGroupMessage,
  ctx: GroupMessageActionContext,
): GroupMessageAction[] {
  if (message.kind === "system" || message.deleted) return [];
  // Not on WhatsApp yet, or never made it: there is nothing there to act on.
  if (message.status === "queued" || message.status === "failed") return [];
  const out = message.direction === "out";
  const mine = out && message.senderUserId === ctx.userId;
  const member = !ctx.group.left;
  const actions: GroupMessageAction[] = [];

  if (ctx.canSend) actions.push("reply");
  if (member) actions.push("react");
  if (!UNFORWARDABLE.has(message.kind)) actions.push("forward");
  actions.push("star");
  if (out) actions.push("info");
  if (
    mine &&
    member &&
    message.kind === "text" &&
    withinEditWindow(message, ctx.now)
  ) {
    actions.push("edit");
  }
  if (member && (mine || (ctx.isAdmin && (out || ctx.group.isAdmin)))) {
    actions.push("revoke");
  }
  if (!out) actions.push("replyPrivate");
  return actions;
}

function withinEditWindow(message: WaGroupMessage, now: number): boolean {
  return now - new Date(message.waTimestamp).getTime() < EDIT_WINDOW_MS;
}

/** What the bubbles call back with. One object rather than nine props, since
 * every bubble passes the same set through untouched. */
export interface GroupMessageHandlers {
  onReply: (message: WaGroupMessage) => void;
  /** The emoji to send; "" takes ours back. */
  onReact: (message: WaGroupMessage, emoji: string) => void;
  onForward: (message: WaGroupMessage) => void;
  onStar: (message: WaGroupMessage) => void;
  onInfo: (message: WaGroupMessage) => void;
  onEdit: (message: WaGroupMessage) => void;
  onRevoke: (message: WaGroupMessage) => void;
  onReplyPrivate: (message: WaGroupMessage) => void;
  onVote: (message: WaGroupMessage, options: string[]) => void;
}
