import { Checkbox, Radio, Typography } from "antd";
import type { WaGroupPoll } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { nextVote, pollTally, type PollOptionTally } from "./groupPoll";

const { Text } = Typography;

function choiceHint(selectableCount: number): string {
  if (selectableCount === 1) return "Pilih satu";
  if (selectableCount > 1) return `Pilih hingga ${selectableCount}`;
  return "Pilih satu atau lebih";
}

function PollOption({
  option,
  single,
  onPick,
}: {
  option: PollOptionTally;
  single: boolean;
  onPick?: () => void;
}) {
  // The row is the control, so a click anywhere on it votes; the antd mark
  // only draws the state and is hidden from assistive tech to avoid a second,
  // dead control under the same name.
  const Mark = single ? Radio : Checkbox;
  return (
    <div
      role={single ? "radio" : "checkbox"}
      aria-checked={option.mine}
      aria-label={option.name}
      aria-disabled={!onPick}
      tabIndex={onPick ? 0 : -1}
      onClick={onPick}
      onKeyDown={(e) => {
        if (onPick && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onPick();
        }
      }}
      style={{ padding: "6px 0", cursor: onPick ? "pointer" : "default" }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span aria-hidden style={{ pointerEvents: "none" }}>
          <Mark checked={option.mine} tabIndex={-1} />
        </span>
        <Text style={{ flex: 1, fontSize: 13 }}>{option.name}</Text>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {option.count}
        </Text>
      </div>
      <div
        style={{
          height: 4,
          margin: "4px 0 0 24px",
          borderRadius: 2,
          background: "rgba(255, 255, 255, 0.08)",
        }}
      >
        <div
          style={{
            width: `${Math.round(option.share * 100)}%`,
            height: "100%",
            borderRadius: 2,
            background: colors.success,
          }}
        />
      </div>
      {option.voterNames.length > 0 && (
        <Text
          type="secondary"
          style={{ display: "block", fontSize: 11, marginLeft: 24 }}
        >
          {option.voterNames.join(", ")}
        </Text>
      )}
    </div>
  );
}

/**
 * A poll as WhatsApp draws it: the question, then each option with its count,
 * its share and who picked it. Clicking an option sends the whole new
 * selection; onVote is absent where our number can no longer vote.
 */
export function PollBubble({
  poll,
  onVote,
}: {
  poll: WaGroupPoll;
  onVote?: (options: string[]) => void;
}) {
  const tally = pollTally(poll);
  const single = poll.selectableCount === 1;
  const pick = (name: string) => {
    const selection = nextVote(poll, name);
    if (selection) onVote?.(selection);
  };

  return (
    <div style={{ minWidth: 220, marginBottom: 4 }}>
      <Text strong style={{ display: "block", fontSize: 14 }}>
        {poll.question}
      </Text>
      <Text type="secondary" style={{ display: "block", fontSize: 11 }}>
        {choiceHint(poll.selectableCount)}
      </Text>
      {tally.options.map((option) => (
        <PollOption
          key={option.name}
          option={option}
          single={single}
          onPick={onVote ? () => pick(option.name) : undefined}
        />
      ))}
      <Text type="secondary" style={{ fontSize: 11 }}>
        {tally.voters} orang memilih
      </Text>
    </div>
  );
}
