import { Typography } from "antd";
import type { WaGroupMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { GroupBubble } from "./GroupBubble";
import {
  dayLabel,
  messageAnchor,
  startsDay,
  startsRun,
} from "./groupThreadLayout";
import type {
  GroupMessageActionContext,
  GroupMessageHandlers,
} from "./groupMessageActions";

const { Text } = Typography;

interface GroupThreadProps {
  /** Newest first, as the API returns them. */
  messages: WaGroupMessage[];
  onRetry: (body: string) => void;
  /** Without both, bubbles carry no menu and nothing can be done to them. */
  context?: Omit<GroupMessageActionContext, "now">;
  handlers?: GroupMessageHandlers;
}

const pill = {
  display: "inline-block",
  maxWidth: "80%",
  padding: "3px 10px",
  borderRadius: 8,
  fontSize: 12,
  background: "rgba(39, 39, 42, 0.92)",
  color: colors.textSecondary,
} as const;

/** A notice WhatsApp wrote itself: centred, grey, no sender and no actions. */
function SystemNotice({ message }: { message: WaGroupMessage }) {
  return (
    <div
      id={messageAnchor(message.id)}
      role="note"
      style={{ textAlign: "center", margin: "8px 0" }}
    >
      <span style={pill}>{message.body}</span>
    </div>
  );
}

function DaySeparator({ iso }: { iso: string }) {
  return (
    <div style={{ textAlign: "center", margin: "10px 0" }}>
      <span style={{ ...pill, color: colors.textMuted }}>{dayLabel(iso)}</span>
    </div>
  );
}

/** A group's messages top to bottom, newest last. */
export function GroupThread({
  messages,
  onRetry,
  context,
  handlers,
}: GroupThreadProps) {
  if (messages.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "48px 24px" }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          Belum ada pesan di grup ini.
        </Text>
      </div>
    );
  }

  // Read once per render, so the edit window is judged against one clock for
  // the whole thread rather than drifting from bubble to bubble.
  const actionContext = context && { ...context, now: Date.now() };
  const ordered = [...messages].reverse();
  return (
    <div style={{ width: "100%" }}>
      {ordered.map((message, i) => {
        const previous = ordered[i - 1];
        return (
          <div key={message.id}>
            {startsDay(message, previous) && (
              <DaySeparator iso={message.waTimestamp} />
            )}
            {message.kind === "system" ? (
              <SystemNotice message={message} />
            ) : (
              <GroupBubble
                message={message}
                startsRun={startsRun(message, previous)}
                onRetry={onRetry}
                context={actionContext}
                handlers={handlers}
              />
            )}
          </div>
        );
      })}
    </div>
  );
}
