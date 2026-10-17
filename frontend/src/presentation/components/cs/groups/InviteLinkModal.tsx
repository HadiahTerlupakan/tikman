import { Modal, Typography } from "antd";

const { Paragraph, Text } = Typography;

/** The invite link an admin asked for, ready to copy. */
export function InviteLinkModal({
  link,
  onClose,
}: {
  link?: string;
  onClose: () => void;
}) {
  return (
    <Modal
      open={!!link}
      title="Tautan undangan grup"
      footer={null}
      onCancel={onClose}
      destroyOnHidden
    >
      <Paragraph copyable={{ text: link, tooltips: ["Salin", "Tersalin"] }}>
        <Text code>{link}</Text>
      </Paragraph>
      <Text type="secondary" style={{ fontSize: 12 }}>
        Siapa pun yang memegang tautan ini bisa bergabung ke grup.
      </Text>
    </Modal>
  );
}
