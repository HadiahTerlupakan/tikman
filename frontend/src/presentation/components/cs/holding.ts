import type { CsConversation } from "@/domain/entities";

/**
 * Whether this CS may reply in a thread: they hold it, or nobody does. Opening
 * a thread makes nobody its holder — the first reply does, so a thread nobody
 * holds is open to whoever answers it. The API enforces the same rule; this
 * only keeps the screen from offering a reply it would refuse.
 */
export function mayReply(
  conversation: CsConversation,
  userId: string,
): boolean {
  return (
    conversation.assignedUserId === userId ||
    conversation.status === "unassigned"
  );
}
