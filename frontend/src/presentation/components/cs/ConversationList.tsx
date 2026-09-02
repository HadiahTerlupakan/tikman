import { Avatar, Badge, List, Typography } from "antd";
import { UserOutlined } from "@ant-design/icons";
import type { CsConversation } from "@/domain/entities";

const { Text } = Typography;

interface ConversationListProps {
  conversations: CsConversation[];
  selectedId?: string;
  holderNames: Record<string, string>;
  currentUserId: string;
  onSelect: (id: string) => void;
}

// Who holds a thread is the first thing a CS needs to see in this list — an
// unlabelled row reads as "nobody's problem" until someone opens it to check.
function holderLabel(
  conversation: CsConversation,
  holderNames: Record<string, string>,
): string {
  if (conversation.status === "unassigned" || !conversation.assignedUserId) {
    return "Belum dipegang";
  }
  return holderNames[conversation.assignedUserId] ?? "Pengguna tidak dikenal";
}

export function ConversationList({
  conversations,
  selectedId,
  holderNames,
  currentUserId,
  onSelect,
}: ConversationListProps) {
  return (
    <List
      dataSource={conversations}
      renderItem={(conversation) => (
        <List.Item
          onClick={() => onSelect(conversation.id)}
          style={{
            cursor: "pointer",
            padding: "8px 12px",
            background:
              conversation.id === selectedId
                ? "rgba(62, 207, 142, 0.08)"
                : undefined,
          }}
        >
          <List.Item.Meta
            avatar={<Avatar icon={<UserOutlined />} />}
            title={
              <span>
                {conversation.customerName || conversation.customerPhone}
                {conversation.assignedUserId === currentUserId && (
                  <Text type="success" style={{ marginLeft: 8, fontSize: 12 }}>
                    (Anda)
                  </Text>
                )}
              </span>
            }
            description={
              <Text type="secondary" style={{ fontSize: 12 }}>
                {holderLabel(conversation, holderNames)}
              </Text>
            }
          />
          <Badge count={conversation.unreadCount} />
        </List.Item>
      )}
    />
  );
}
