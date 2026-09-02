import { Avatar, Badge, Typography } from "antd";
import { UserOutlined } from "@ant-design/icons";
import type { CsConversation, CsLastMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

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
  if (conversation.status === "unassigned" || !conversation.assignedUserId) {
    return "Belum dipegang";
  }
  if (conversation.assignedUserId === currentUserId) return "Anda";
  return holderNames[conversation.assignedUserId] ?? "Pengguna tidak dikenal";
}

// Media arrives with an empty body, so a preview built from the body alone
// would show a blank line where a photo is the whole message.
const kindLabels: Record<CsLastMessage["kind"], string> = {
  text: "",
  image: "Foto",
  document: "Dokumen",
  audio: "Pesan suara",
  video: "Video",
};

function preview(last?: CsLastMessage): string {
  if (!last) return "Belum ada pesan";
  const label = kindLabels[last.kind];
  const body = last.body.trim();
  if (label) return body ? `${label} · ${body}` : label;
  return body || "Pesan kosong";
}

// Today shows a clock, anything older shows a date — the same shorthand every
// messaging app uses, because a bare timestamp on a week-old thread is noise.
function shortTime(iso: string): string {
  const at = new Date(iso);
  const sameDay = new Date().toDateString() === at.toDateString();
  return at.toLocaleString("id-ID", {
    ...(sameDay
      ? { hour: "2-digit", minute: "2-digit" }
      : { day: "2-digit", month: "short" }),
  });
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
            <Avatar size={42} icon={<UserOutlined />} />

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
            </div>
          </div>
        );
      })}
    </div>
  );
}
