import { Badge, Dropdown, Empty, Typography } from "antd";
import { BellOutlined } from "@ant-design/icons";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { CustomerAvatar } from "./CustomerAvatar";
import { preview, shortTime } from "./conversationSummary";

const { Text } = Typography;

/** How many threads the panel lists before deferring to the inbox. A bell is a
 * glance, not a second inbox — past this the list stops being scannable and the
 * "lihat semua" footer is the better answer. */
const MAX_LISTED = 6;

interface NotificationBellProps {
  /** The threads waiting on a CS reply — the same set the badge counts. */
  conversations: CsConversation[];
  /** Opens one thread in the inbox. */
  onOpen: (conversationId: string) => void;
  /** Opens the inbox filtered to everything waiting. */
  onSeeAll: () => void;
}

/**
 * The navbar bell. It opens its own panel rather than jumping straight to the
 * inbox: a count with no detail makes a CS switch pages to find out whether it
 * is worth switching pages.
 *
 * It is deliberately not inside the avatar's dropdown trigger — it was once,
 * back when the badge was hardcoded to zero and nobody clicked it, and the
 * first click anyone tried opened the user menu instead.
 */
export function NotificationBell({
  conversations,
  onOpen,
  onSeeAll,
}: NotificationBellProps) {
  const listed = conversations.slice(0, MAX_LISTED);
  const remaining = conversations.length - listed.length;

  const panel = (
    <div
      style={{
        width: 340,
        background: colors.surface,
        border: `1px solid ${colors.border}`,
        borderRadius: 10,
        overflow: "hidden",
      }}
    >
      <div
        style={{
          padding: "10px 14px",
          borderBottom: `1px solid ${colors.border}`,
          fontSize: 13,
          fontWeight: 500,
        }}
      >
        Menunggu dibalas ({conversations.length})
      </div>

      {conversations.length === 0 ? (
        <div style={{ padding: "18px 14px" }}>
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="Semua sudah dibalas"
          />
        </div>
      ) : (
        <div style={{ maxHeight: 360, overflowY: "auto" }}>
          {listed.map((conversation) => (
            <div
              key={conversation.id}
              role="button"
              tabIndex={0}
              onClick={() => onOpen(conversation.id)}
              onKeyDown={(event) => {
                if (event.key === "Enter" || event.key === " ") {
                  event.preventDefault();
                  onOpen(conversation.id);
                }
              }}
              style={{
                display: "flex",
                gap: 10,
                alignItems: "center",
                padding: "10px 14px",
                cursor: "pointer",
                borderBottom: `1px solid ${colors.border}`,
              }}
            >
              <CustomerAvatar conversation={conversation} size={32} />
              <div style={{ minWidth: 0, flex: 1 }}>
                <div
                  style={{
                    display: "flex",
                    justifyContent: "space-between",
                    gap: 8,
                  }}
                >
                  <Text ellipsis style={{ fontSize: 13, fontWeight: 500 }}>
                    {conversation.customerName || conversation.customerPhone}
                  </Text>
                  <Text type="secondary" style={{ fontSize: 11 }}>
                    {shortTime(
                      conversation.lastMessage?.at ??
                        conversation.lastMessageAt,
                    )}
                  </Text>
                </div>
                <Text
                  type="secondary"
                  ellipsis
                  style={{ fontSize: 12, display: "block" }}
                >
                  {preview(conversation.lastMessage)}
                </Text>
              </div>
            </div>
          ))}
        </div>
      )}

      <div
        role="button"
        tabIndex={0}
        onClick={onSeeAll}
        onKeyDown={(event) => {
          if (event.key === "Enter" || event.key === " ") {
            event.preventDefault();
            onSeeAll();
          }
        }}
        style={{
          padding: "10px 14px",
          textAlign: "center",
          cursor: "pointer",
          fontSize: 13,
          color: colors.success,
        }}
      >
        {remaining > 0 ? `Lihat semua (${remaining} lagi)` : "Buka CS Inbox"}
      </div>
    </div>
  );

  return (
    <Dropdown
      dropdownRender={() => panel}
      trigger={["click"]}
      placement="bottomRight"
    >
      <span
        role="button"
        aria-label="Percakapan menunggu dibalas"
        style={{
          display: "inline-flex",
          alignItems: "center",
          cursor: "pointer",
          padding: 4,
        }}
      >
        {/* Ant Design puts the count in a native title attribute, which the
            browser renders as a second tooltip beside the panel. */}
        <Badge count={conversations.length} title="">
          <BellOutlined
            style={{ fontSize: 18, color: "#a1a1aa", display: "block" }}
          />
        </Badge>
      </span>
    </Dropdown>
  );
}
