import { useEffect, useState } from "react";
import { Alert, Button, Input, Modal, Typography } from "antd";
import type { WaAccountStatus } from "@/domain/entities";

const { Text, Title } = Typography;

interface WaPairingModalProps {
  open: boolean;
  onClose: () => void;
  status?: WaAccountStatus;
  pairingCode?: string;
  connecting: boolean;
  onConnect: (phone: string) => void;
}

/**
 * There is no QR renderer in this project, and an eight-character code does
 * not need one: the admin types it into WhatsApp by hand, on the phone that
 * actually holds the number.
 */
export function WaPairingModal({
  open,
  onClose,
  status,
  pairingCode,
  connecting,
  onConnect,
}: WaPairingModalProps) {
  const [phone, setPhone] = useState("");

  useEffect(() => {
    if (!open) setPhone("");
  }, [open]);

  return (
    <Modal
      title="Sambungkan Nomor WhatsApp"
      open={open}
      onCancel={onClose}
      footer={null}
    >
      {pairingCode ? (
        <div style={{ textAlign: "center", padding: "8px 0" }}>
          <Text type="secondary">Masukkan kode ini di WhatsApp</Text>
          <Title level={2} style={{ letterSpacing: 6, margin: "12px 0" }}>
            {pairingCode}
          </Title>
          <Text type="secondary">
            Buka WhatsApp di HP yang memegang nomor ini, masuk ke Perangkat
            Tertaut, pilih &quot;Tautkan dengan nomor telepon&quot;, lalu ketik
            kode di atas.
          </Text>
        </div>
      ) : (
        <>
          <Input
            placeholder="Nomor WhatsApp CS, contoh 628111222333"
            value={phone}
            onChange={(e) => setPhone(e.target.value)}
            style={{ marginBottom: 12 }}
          />
          <Button
            type="primary"
            block
            loading={connecting}
            disabled={!phone.trim()}
            onClick={() => onConnect(phone.trim())}
          >
            Sambungkan
          </Button>
          {status === "pairing" && (
            <Alert
              style={{ marginTop: 12 }}
              type="info"
              showIcon
              message="Menunggu kode dari WhatsApp..."
            />
          )}
        </>
      )}
    </Modal>
  );
}
