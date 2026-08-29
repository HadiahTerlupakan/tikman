import { useEffect, useState } from "react";
import { Alert, Modal, Tabs, Typography } from "antd";
import { usePeerConfig } from "@/application/hooks";
import type { PeerConfigFormat } from "@/domain/entities";

interface Props {
  peerId: string | null;
  onClose: () => void;
}

export function VpnConfigModal({ peerId, onClose }: Props) {
  const [format, setFormat] = useState<PeerConfigFormat>("mikrotik");
  const peerConfig = usePeerConfig();
  const { mutate } = peerConfig;

  useEffect(() => {
    if (peerId) {
      mutate({ id: peerId, format });
    }
  }, [peerId, format, mutate]);

  return (
    <Modal
      open={!!peerId}
      title="Konfigurasi untuk perangkat di site"
      footer={null}
      width={720}
      onCancel={onClose}
    >
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Berisi kunci privat"
        description="Tempel hanya ke perangkat di site tersebut, jangan dibagikan lewat kanal terbuka."
      />
      <Tabs
        activeKey={format}
        onChange={(key) => setFormat(key as PeerConfigFormat)}
        items={[
          { key: "mikrotik", label: "MikroTik" },
          { key: "wg-quick", label: "Linux (wg-quick)" },
        ]}
      />
      <Typography.Paragraph copyable={{ text: peerConfig.data?.config ?? "" }}>
        <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>
          {peerConfig.isPending ? "Menyiapkan..." : peerConfig.data?.config}
        </pre>
      </Typography.Paragraph>
    </Modal>
  );
}
