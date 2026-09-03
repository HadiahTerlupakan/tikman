import { useEffect, useState } from "react";
import { Alert, Input, Modal, Space, Typography } from "antd";

const { Text } = Typography;

interface ConfirmByTypingModalProps {
  open: boolean;
  title: string;
  /** What is about to happen, in the words of what will be lost. */
  warning: string;
  /** The exact text the admin has to type. Shown to them, and compared
   * literally — this is the one gesture standing between a click and history
   * nobody can get back. */
  phrase: string;
  confirmText: string;
  loading?: boolean;
  onConfirm: () => void;
  onClose: () => void;
}

/**
 * A confirmation for the removals that cannot be undone.
 *
 * A Popconfirm is right for one message, where a misclick costs one line. It is
 * not right for a whole number or a whole inbox: those are one careless Enter
 * away from years of history, and there is no copy of it anywhere else. Typing
 * the name back makes the admin read what they are about to do.
 */
export function ConfirmByTypingModal({
  open,
  title,
  warning,
  phrase,
  confirmText,
  loading,
  onConfirm,
  onClose,
}: ConfirmByTypingModalProps) {
  const [typed, setTyped] = useState("");

  // Cleared on open rather than on close: a modal that reopens still carrying
  // the last confirmation would let the second one through on one click.
  useEffect(() => {
    if (open) setTyped("");
  }, [open]);

  const matches = typed.trim() === phrase;

  return (
    <Modal
      open={open}
      title={title}
      okText={confirmText}
      okButtonProps={{ danger: true, disabled: !matches, loading }}
      cancelText="Batal"
      onOk={onConfirm}
      onCancel={onClose}
      width={460}
    >
      <Space direction="vertical" style={{ width: "100%" }} size="middle">
        <Alert type="warning" showIcon message={warning} />
        <div>
          <Text type="secondary">
            Ketik <Text strong>{phrase}</Text> untuk melanjutkan.
          </Text>
          <Input
            value={typed}
            onChange={(e) => setTyped(e.target.value)}
            onPressEnter={() => matches && onConfirm()}
            placeholder={phrase}
            style={{ marginTop: 8 }}
          />
        </div>
      </Space>
    </Modal>
  );
}
