import { Button, Popconfirm, Space, Tag, Typography } from "antd";
import { ClearOutlined } from "@ant-design/icons";
import { CustomerAvatar } from "./CustomerAvatar";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface ThreadHeaderProps {
  conversation: CsConversation;
  holderName?: string;
  isHolder: boolean;
  /** Empties this thread's history. Absent for anyone who may not — the same
   * gate as replying, because the CS working a thread is the one who can tell
   * a mistake from the customer's own words. */
  onClear?: () => void;
  clearing?: boolean;
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
  onClear,
  clearing,
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
        <Space size={6}>
          <Text style={{ color: colors.textMuted, fontSize: 12 }}>
            {conversation.customerPhone}
          </Text>
          {/* A CS about to type needs to know which of our numbers this
              customer is talking to — the reply leaves from that one. */}
          {conversation.waAccountLabel && (
            <Tag bordered={false} style={{ fontSize: 10, margin: 0 }}>
              {conversation.waAccountLabel}
            </Tag>
          )}
        </Space>
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

      {onClear && (
        <Popconfirm
          title="Bersihkan semua pesan?"
          description="Seluruh riwayat percakapan ini dihapus di TikMan. Pesan di HP pelanggan tetap ada."
          okText="Bersihkan"
          okButtonProps={{ danger: true }}
          cancelText="Batal"
          onConfirm={onClear}
        >
          <Button
            type="text"
            size="small"
            danger
            loading={clearing}
            icon={<ClearOutlined />}
            aria-label="Bersihkan pesan percakapan ini"
            title="Bersihkan pesan"
          />
        </Popconfirm>
      )}
    </div>
  );
}
