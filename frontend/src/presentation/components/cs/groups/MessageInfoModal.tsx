import { Empty, List, Modal, Spin, Typography } from "antd";
import { useWaGroupReceipts } from "@/application/hooks";
import type { WaGroupMessage, WaGroupReceipt } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

function when(iso: string): string {
  return new Date(iso).toLocaleString("id-ID", {
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function ReceiptSection({
  title,
  receipts,
  at,
}: {
  title: string;
  receipts: WaGroupReceipt[];
  at: (r: WaGroupReceipt) => string | undefined;
}) {
  if (receipts.length === 0) return null;
  return (
    <>
      <Text
        style={{
          display: "block",
          fontSize: 12,
          color: colors.textMuted,
          margin: "8px 0 2px",
        }}
      >
        {title} ({receipts.length})
      </Text>
      <List
        size="small"
        dataSource={receipts}
        renderItem={(r) => {
          const time = at(r);
          return (
            <List.Item
              extra={
                time && (
                  <Text type="secondary" style={{ fontSize: 12 }}>
                    {when(time)}
                  </Text>
                )
              }
            >
              {r.name || r.jid.split("@")[0]}
            </List.Item>
          );
        }}
      />
    </>
  );
}

/**
 * WhatsApp's "Info pesan" for one of our messages: who has read it, who has
 * it on their phone but not yet open, and who it has not reached.
 */
export function MessageInfoModal({
  message,
  onClose,
}: {
  message?: WaGroupMessage;
  onClose: () => void;
}) {
  const receipts = useWaGroupReceipts(message?.groupId, message?.id);
  const all = receipts.data ?? [];
  const read = all.filter((r) => r.readAt);
  const delivered = all.filter((r) => !r.readAt && r.deliveredAt);
  const waiting = all.filter((r) => !r.readAt && !r.deliveredAt);

  return (
    <Modal
      open={!!message}
      title="Info pesan"
      footer={null}
      onCancel={onClose}
      destroyOnHidden
    >
      {receipts.isLoading ? (
        <Spin />
      ) : all.length === 0 ? (
        <Empty description="Belum ada laporan dari anggota" />
      ) : (
        <>
          <ReceiptSection
            title="Dibaca oleh"
            receipts={read}
            at={(r) => r.readAt}
          />
          <ReceiptSection
            title="Terkirim ke"
            receipts={delivered}
            at={(r) => r.deliveredAt}
          />
          <ReceiptSection
            title="Belum sampai"
            receipts={waiting}
            at={() => undefined}
          />
        </>
      )}
    </Modal>
  );
}
