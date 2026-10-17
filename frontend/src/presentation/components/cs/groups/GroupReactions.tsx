import { Tooltip } from "antd";
import type { WaGroupReaction } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { reactionToSend, reactorNames } from "./reactionChoice";

/**
 * The emoji chips under a bubble, one per emoji with how many chose it. Ours
 * is outlined; clicking a chip toggles that emoji as ours, as tapping one does
 * on the phone. onReact is absent where our number can no longer react.
 */
export function GroupReactions({
  reactions,
  onReact,
}: {
  reactions: WaGroupReaction[];
  onReact?: (emoji: string) => void;
}) {
  if (reactions.length === 0) return null;
  return (
    <div style={{ display: "flex", flexWrap: "wrap", gap: 4, marginTop: -2 }}>
      {reactions.map((reaction) => {
        const mine = reaction.reactors.some((r) => r.isSelf);
        return (
          <Tooltip key={reaction.emoji} title={reactorNames(reaction)}>
            <button
              type="button"
              aria-pressed={mine}
              aria-label={`${reaction.emoji} ${reaction.count}`}
              disabled={!onReact}
              onClick={() =>
                onReact?.(reactionToSend(reactions, reaction.emoji))
              }
              style={{
                display: "inline-flex",
                alignItems: "center",
                gap: 3,
                padding: "1px 7px",
                fontSize: 12,
                lineHeight: "18px",
                borderRadius: 12,
                cursor: onReact ? "pointer" : "default",
                color: colors.textBody,
                background: "#303034",
                border: `1px solid ${mine ? colors.success : colors.border}`,
              }}
            >
              <span>{reaction.emoji}</span>
              {reaction.count > 1 && <span>{reaction.count}</span>}
            </button>
          </Tooltip>
        );
      })}
    </div>
  );
}
