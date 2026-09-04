import { Button, Popconfirm, Space, Tag, Typography } from "antd";
import { ArrowLeftOutlined, ClearOutlined } from "@ant-design/icons";
import { CustomerAvatar } from "./CustomerAvatar";
import type { CsConversation } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface ThreadHeaderProps {
  conversation: CsConversation;
  holderName?: string;
  isHolder: boolean;
  /** True while the customer is writing. Shown in place of their number,
   * because that is where a chat app puts it and a CS reads it without
   * looking for it. */
  typing?: boolean;
  /** Empties this thread's history. Absent for anyone who may not — the same
   * gate as replying, because the CS working a thread is the one who can tell
   * a mistake from the customer's own words. */
  onClear?: () => void;
  clearing?: boolean;
  /** Returns to the conversation list. Present only where the panes take
   * turns; on a desktop the list is already beside this one. */
  onBack?: () => void;
  /** Opens the customer's details. Present for the same reason as onBack, and
   * reached by tapping the name because that is where a chat app puts it. */
  onOpenCustomer?: () => void;
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
  typing = false,
  onClear,
  clearing,
  onBack,
  onOpenCustomer,
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
      {onBack && (
        <Button
          type="text"
          aria-label="Kembali ke daftar percakapan"
          icon={<ArrowLeftOutlined />}
          onClick={onBack}
        />
      )}

      <CustomerAvatar conversation={conversation} size={38} />

      <div
        // A button only where there is somewhere to go: on a desktop the
        // customer's details already sit in the third column.
        role={onOpenCustomer ? "button" : undefined}
        tabIndex={onOpenCustomer ? 0 : undefined}
        onClick={onOpenCustomer}
        onKeyDown={(e) => {
          if (onOpenCustomer && (e.key === "Enter" || e.key === " ")) {
            e.preventDefault();
            onOpenCustomer();
          }
        }}
        style={{
          flex: 1,
          minWidth: 0,
          cursor: onOpenCustomer ? "pointer" : undefined,
        }}
      >
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
          {typing ? (
            <Text style={{ color: colors.success, fontSize: 12 }}>
              sedang mengetik…
            </Text>
          ) : (
            <Text style={{ color: colors.textMuted, fontSize: 12 }}>
              {conversation.customerPhone}
            </Text>
          )}
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
