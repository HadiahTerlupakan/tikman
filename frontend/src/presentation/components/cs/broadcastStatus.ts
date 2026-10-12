import type { BroadcastPost } from "@/domain/entities";

/** A status disappears from WhatsApp after a day. Computed rather than
 * stored: a column would claim we know something we only calculate. */
const STATUS_LIFETIME_MS = 24 * 60 * 60 * 1000;

/** Whether a sent status is past the day WhatsApp shows it for. Nothing else
 * decides whether a row still reads as live. */
export function statusHasExpired(
  post: BroadcastPost,
  now = Date.now(),
): boolean {
  if (post.destination !== "status" || !post.sentAt) return false;
  return now - new Date(post.sentAt).getTime() > STATUS_LIFETIME_MS;
}
