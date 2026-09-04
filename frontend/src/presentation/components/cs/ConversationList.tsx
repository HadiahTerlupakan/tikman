import { Badge, Tag, Typography } from "antd";
import { CustomerAvatar } from "./CustomerAvatar";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { preview, shortTime } from "./conversationSummary";

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
  currentUserId: string,
): string {
  // "Semua" includes finished threads, so one has to say it is finished —
  // otherwise it sits in the list looking like work nobody has done.
  if (conversation.status === "closed") return "Selesai";
  if (conversation.status === "unassigned" || !conversation.assignedUserId) {
    return "Belum dipegang";
  }
  if (conversation.assignedUserId === currentUserId) return "Anda";
  return holderNames[conversation.assignedUserId] ?? "Pengguna tidak dikenal";
}

export function ConversationList({
  conversations,
  selectedId,
  holderNames,
  currentUserId,
  onSelect,
}: ConversationListProps) {
  if (conversations.length === 0) {
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          Belum ada percakapan. Pesan yang masuk ke nomor CS akan muncul di
          sini.
        </Text>
      </div>
    );
  }

  return (
    <div role="list">
      {conversations.map((conversation) => {
        const selected = conversation.id === selectedId;
        const unread = conversation.unreadCount > 0;
        return (
          <div
            key={conversation.id}
            role="listitem"
            tabIndex={0}
            onClick={() => onSelect(conversation.id)}
            onKeyDown={(e) => {
              if (e.key === "Enter" || e.key === " ") {
                e.preventDefault();
                onSelect(conversation.id);
              }
            }}
            style={{
              display: "flex",
              gap: 12,
              alignItems: "center",
              padding: "10px 14px",
              cursor: "pointer",
              borderBottom: `1px solid ${colors.border}`,
              background: selected ? "rgba(62, 207, 142, 0.08)" : undefined,
              borderLeft: selected
                ? `2px solid ${colors.success}`
                : "2px solid transparent",
            }}
          >
            <CustomerAvatar conversation={conversation} size={42} />

            <div style={{ flex: 1, minWidth: 0 }}>
              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  gap: 8,
                }}
              >
                <Text
                  strong={unread}
                  style={{
                    color: colors.textPrimary,
                    fontSize: 14,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {conversation.customerName || conversation.customerPhone}
                </Text>
                <Text
                  style={{
                    fontSize: 11,
                    whiteSpace: "nowrap",
                    color: unread ? colors.success : colors.textMuted,
                  }}
                >
                  {shortTime(
                    conversation.lastMessage?.at ?? conversation.lastMessageAt,
                  )}
                </Text>
              </div>

              <div
                style={{
                  display: "flex",
                  justifyContent: "space-between",
                  gap: 8,
                  alignItems: "center",
                  marginTop: 2,
                }}
              >
                <Text
                  style={{
                    fontSize: 13,
                    color: unread ? colors.textBody : colors.textSecondary,
                    overflow: "hidden",
                    textOverflow: "ellipsis",
                    whiteSpace: "nowrap",
                  }}
                >
                  {conversation.lastMessage?.direction === "out" && "Anda: "}
                  {preview(conversation.lastMessage)}
                </Text>
                <Badge
                  count={conversation.unreadCount}
                  style={{ backgroundColor: colors.success, color: "#0b1f16" }}
                />
              </div>

              <div
                style={{
                  display: "flex",
                  alignItems: "center",
                  gap: 6,
                  minWidth: 0,
                }}
              >
                <Text
                  style={{
                    fontSize: 11,
                    color:
                      conversation.status === "unassigned"
                        ? colors.warning
                        : colors.textMuted,
                  }}
                >
                  {holderLabel(conversation, holderNames, currentUserId)}
                </Text>
                {/* Which of our numbers the customer wrote to. Rendered only
                    when the API names one, so an inbox on a single number is
                    not given a label that says nothing. */}
                {conversation.waAccountLabel && (
                  <Tag
                    bordered={false}
                    style={{
                      fontSize: 10,
                      lineHeight: "16px",
                      margin: 0,
                      padding: "0 5px",
                    }}
                  >
                    {conversation.waAccountLabel}
                  </Tag>
                )}
              </div>
            </div>
          </div>
        );
      })}
    </div>
  );
}
