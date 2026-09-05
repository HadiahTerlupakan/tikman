import { Empty, Typography } from "antd";
import type { CsConversation, User } from "@/domain/entities";
import { CustomerPanel } from "./CustomerPanel";
import { CsTeamPanel } from "./CsTeamPanel";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface CustomerPaneProps {
  conversation?: CsConversation;
  users: User[];
  online: string[];
  currentUserId?: string;
  notice?: { text: string; color: string };
}

/**
 * The third column: who the customer is, above who from the team is at the
 * inbox.
 */
export function CustomerPane({
  conversation,
  users,
  online,
  currentUserId,
  notice,
}: CustomerPaneProps) {
  return (
    <>
      {/* The placeholder is centred and the real panel is not: pinned to the
          top, "Data pelanggan muncul di sini" left the column looking broken
          rather than empty. A loaded panel scrolls from the top as it always
          did. */}
      <div
        style={{
          flex: 1,
          overflowY: "auto",
          padding: 14,
          display: conversation ? "block" : "flex",
          alignItems: conversation ? undefined : "center",
          justifyContent: conversation ? undefined : "center",
        }}
      >
        {conversation ? (
          <CustomerPanel conversation={conversation} />
        ) : (
          <Empty
            image={Empty.PRESENTED_IMAGE_SIMPLE}
            description="Data pelanggan muncul di sini"
          />
        )}
      </div>

      {/* Fixed height rather than shared: who is at the inbox is ambient, and
          letting it grow would push the subscriber's details — the reason the
          column exists — off the screen. */}
      <div
        style={{
          borderTop: `1px solid ${colors.border}`,
          maxHeight: 200,
          overflowY: "auto",
          flexShrink: 0,
        }}
      >
        {notice && (
          <Text
            style={{
              display: "block",
              padding: "8px 14px 0",
              color: notice.color,
              fontSize: 11,
            }}
          >
            {notice.text}
          </Text>
        )}
        <CsTeamPanel
          users={users}
          online={online}
          currentUserId={currentUserId}
        />
      </div>
    </>
  );
}
