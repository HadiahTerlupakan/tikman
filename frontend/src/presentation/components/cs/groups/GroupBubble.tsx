import { useState } from "react";
import { Alert, Button, Typography } from "antd";
import { RedoOutlined, StarFilled } from "@ant-design/icons";
import type { WaGroupMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { DeliveryMark } from "../DeliveryMark";
import { MemberAvatar } from "./GroupAvatars";
import { GroupBubbleBody } from "./GroupBubbleBody";
import { GroupMessageMenu } from "./GroupMessageMenu";
import { GroupReactions } from "./GroupReactions";
import { senderLabel } from "./groupSummary";
import { messageAnchor } from "./groupThreadLayout";
import { senderColor } from "./senderColor";
import type {
  GroupMessageActionContext,
  GroupMessageHandlers,
} from "./groupMessageActions";

const { Text } = Typography;

const AVATAR_SIZE = 28;

function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function BubbleFooter({
  message,
  onRetry,
}: {
  message: WaGroupMessage;
  onRetry: (body: string) => void;
}) {
  const outgoing = message.direction === "out";
  return (
    <>
      <div
        style={{
          display: "flex",
          justifyContent: "flex-end",
          alignItems: "center",
          gap: 4,
          marginTop: 2,
          color: colors.textMuted,
          fontSize: 11,
        }}
      >
        {message.starred && (
          <StarFilled aria-label="Berbintang" style={{ fontSize: 10 }} />
        )}
        {outgoing && !message.senderUserId && <span>dari HP</span>}
        {message.edited && !message.deleted && <span>Diedit</span>}
        <span>{clock(message.waTimestamp)}</span>
        {outgoing && !message.deleted && (
          <DeliveryMark
            status={message.status}
            deliveredLabel="Sampai di HP anggota"
          />
        )}
      </div>
      {message.status === "failed" && (
        <Alert
          type="error"
          showIcon
          message={message.failReason || "Gagal terkirim"}
          action={
            message.kind === "text" && (
              <Button
                size="small"
                icon={<RedoOutlined />}
                onClick={() => onRetry(message.body)}
              >
                Coba lagi
              </Button>
            )
          }
          style={{ marginTop: 6 }}
        />
      )}
    </>
  );
}

interface GroupBubbleProps {
  message: WaGroupMessage;
  /** First of a run by one member: named, and with their photo beside it. */
  startsRun: boolean;
  onRetry: (body: string) => void;
  /** Both absent where the thread is only being read. */
  context?: GroupMessageActionContext;
  handlers?: GroupMessageHandlers;
}

function SenderAvatar({
  message,
  shown,
}: {
  message: WaGroupMessage;
  shown: boolean;
}) {
  if (!shown) return <span style={{ width: AVATAR_SIZE, flexShrink: 0 }} />;
  return (
    <MemberAvatar
      groupId={message.groupId}
      jid={message.senderJid}
      hasAvatar={!!message.senderHasAvatar}
      size={AVATAR_SIZE}
      color={senderColor(message.senderJid)}
    />
  );
}

export function GroupBubble({
  message,
  startsRun,
  onRetry,
  context,
  handlers,
}: GroupBubbleProps) {
  const outgoing = message.direction === "out";
  const [hovered, setHovered] = useState(false);
  const member = !!context && !context.group.left;
  const menu = context && handlers && (
    <GroupMessageMenu
      message={message}
      context={context}
      handlers={handlers}
      visible={hovered}
    />
  );

  return (
    <div
      id={messageAnchor(message.id)}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 6,
        justifyContent: outgoing ? "flex-end" : "flex-start",
        marginBottom: 6,
        marginTop: startsRun ? 8 : 0,
      }}
    >
      {!outgoing && <SenderAvatar message={message} shown={startsRun} />}
      {outgoing && menu}
      <div style={{ maxWidth: "68%", minWidth: 96 }}>
        <div
          style={{
            padding: "7px 10px 5px",
            background: outgoing ? "rgba(62, 207, 142, 0.14)" : "#27272a",
            borderRadius: 10,
            borderTopLeftRadius: !outgoing && startsRun ? 2 : 10,
            borderTopRightRadius: outgoing ? 2 : 10,
          }}
        >
          {startsRun && (
            <Text
              strong
              style={{
                display: "block",
                fontSize: 12,
                color: senderColor(message.senderJid),
              }}
            >
              {senderLabel(message)}
            </Text>
          )}
          <GroupBubbleBody
            message={message}
            onVote={
              member && handlers
                ? (options) => handlers.onVote(message, options)
                : undefined
            }
          />
          <BubbleFooter message={message} onRetry={onRetry} />
        </div>
        {!message.deleted && (
          <GroupReactions
            reactions={message.reactions ?? []}
            onReact={
              member && handlers
                ? (emoji) => handlers.onReact(message, emoji)
                : undefined
            }
          />
        )}
      </div>
      {!outgoing && menu}
    </div>
  );
}
