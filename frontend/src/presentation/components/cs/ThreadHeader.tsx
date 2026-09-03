import { Tag, Typography } from "antd";
import { CustomerAvatar } from "./CustomerAvatar";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface ThreadHeaderProps {
  conversation: CsConversation;
  holderName?: string;
  isHolder: boolean;
}

/**
 * The band above a thread, answering the two things a CS needs before typing:
 * who this is, and whether the reply is theirs to send. Without it the middle
 * column opened straight into bubbles, and the customer's number lived only in
 * the list they had just clicked away from.
 */
export function ThreadHeader({
  conversation,
  holderName,
  isHolder,
}: ThreadHeaderProps) {
  const unheld =
    conversation.status === "unassigned" || !conversation.assignedUserId;

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        gap: 12,
        padding: "10px 14px",
        borderBottom: `1px solid ${colors.border}`,
        background: colors.surface,
      }}
    >
      <CustomerAvatar conversation={conversation} size={38} />

      <div style={{ flex: 1, minWidth: 0 }}>
        <Text
          strong
          style={{
            color: colors.textPrimary,
            fontSize: 14,
            display: "block",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {conversation.customerName || conversation.customerPhone}
        </Text>
        <Text style={{ color: colors.textMuted, fontSize: 12 }}>
          {conversation.customerPhone}
        </Text>
      </div>

      {conversation.status === "closed" ? (
        <Tag>Selesai</Tag>
      ) : unheld ? (
        <Tag color="warning">Belum dipegang</Tag>
      ) : isHolder ? (
        <Tag color="success">Anda yang pegang</Tag>
      ) : (
        <Tag>Dipegang {holderName ?? "CS lain"}</Tag>
      )}
    </div>
  );
}
