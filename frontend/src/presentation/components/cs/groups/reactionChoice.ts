import type { WaGroupReaction } from "@/domain/entities";

/** WhatsApp's own quick row. */
export const QUICK_REACTIONS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

/** The emoji our number reacted with, or null. WhatsApp allows one each. */
export function ownReaction(reactions: WaGroupReaction[]): string | null {
  const mine = reactions.find((r) => r.reactors.some((p) => p.isSelf));
  return mine?.emoji ?? null;
}

/**
 * What to send when an emoji is picked or its chip clicked: picking the one we
 * already chose takes it back (""), anything else replaces it — the toggle
 * WhatsApp does on a tap.
 */
export function reactionToSend(
  reactions: WaGroupReaction[],
  emoji: string,
): string {
  return ownReaction(reactions) === emoji ? "" : emoji;
}

/** Who reacted with one emoji, for the chip's tooltip. */
export function reactorNames(reaction: WaGroupReaction): string {
  return reaction.reactors.map((r) => (r.isSelf ? "Anda" : r.name)).join(", ");
}
