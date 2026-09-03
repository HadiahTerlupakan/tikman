import { useState } from "react";
import { Button, Popconfirm } from "antd";
import { DeleteOutlined, EnterOutlined } from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";

/**
 * The buttons that appear on the message being pointed at.
 *
 * WhatsApp swipes to reply and long-presses to delete; this is an inbox worked
 * with a mouse, so both are buttons that surface on hover. They keep their
 * space when hidden, or every bubble would shift as the pointer crosses the
 * thread.
 */
export function BubbleActions({
  message,
  visible,
  onReply,
  onDelete,
}: {
  message: CsMessage;
  visible: boolean;
  onReply?: (message: CsMessage) => void;
  onDelete?: (message: CsMessage) => void;
}) {
  if (!onReply && !onDelete) return null;

  return (
    <div style={{ display: "flex", flexShrink: 0 }}>
      {onReply && (
        <HoverAction
          visible={visible}
          icon={<EnterOutlined style={{ transform: "scaleX(-1)" }} />}
          label="Balas pesan ini"
          title="Balas"
          onClick={() => onReply(message)}
        />
      )}
      {onDelete && (
        // Confirmed, and worded so nobody expects more than it does: the copy
        // on the customer's phone is not ours to take back.
        <Popconfirm
          title="Hapus pesan ini?"
          description="Hanya terhapus di TikMan. Pesan di HP pelanggan tetap ada."
          okText="Hapus"
          okButtonProps={{ danger: true }}
          cancelText="Batal"
          onConfirm={() => onDelete(message)}
        >
          <HoverAction
            visible={visible}
            danger
            icon={<DeleteOutlined />}
            label="Hapus pesan ini"
            title="Hapus"
          />
        </Popconfirm>
      )}
    </div>
  );
}

/**
 * One hover-revealed button.
 *
 * Hidden by opacity rather than visibility, and revealed by focus as well as
 * hover: visibility takes the button out of the accessibility tree entirely,
 * which left these actions reachable by mouse alone.
 */
function HoverAction({
  visible,
  icon,
  label,
  title,
  danger,
  onClick,
}: {
  visible: boolean;
  icon: React.ReactNode;
  label: string;
  title: string;
  danger?: boolean;
  onClick?: () => void;
}) {
  const [focused, setFocused] = useState(false);

  return (
    <Button
      type="text"
      size="small"
      danger={danger}
      icon={icon}
      onClick={onClick}
      onFocus={() => setFocused(true)}
      onBlur={() => setFocused(false)}
      aria-label={label}
      title={title}
      style={{
        opacity: visible || focused ? 1 : 0,
        transition: "opacity 120ms",
      }}
    />
  );
}
