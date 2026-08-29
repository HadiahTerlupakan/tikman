import { useEffect, useState } from "react";
import { Alert, Modal, Tabs, Typography } from "antd";
import { usePeerConfig } from "@/application/hooks";
import type { PeerConfigFormat } from "@/domain/entities";
import { clientSteps } from "./vpnClientSteps";

interface Props {
  peerId: string | null;
  onClose: () => void;
}

export function VpnConfigModal({ peerId, onClose }: Props) {
  const [format, setFormat] = useState<PeerConfigFormat>("mikrotik");
  const peerConfig = usePeerConfig();
  const { mutate, reset } = peerConfig;

  useEffect(() => {
    // Reset before fetching: the previous peer's config carries its private key
    // and must not stay on screen under another peer's name.
    reset();
    if (!peerId) {
      return;
    }
    mutate({ id: peerId, format });
  }, [peerId, format, mutate, reset]);

  const steps = clientSteps(format);

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
      {peerConfig.isError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="Gagal menyiapkan konfigurasi"
          description={(peerConfig.error as Error).message}
        />
      )}
      <Tabs
        activeKey={format}
        onChange={(key) => setFormat(key as PeerConfigFormat)}
        items={[
          { key: "mikrotik", label: "MikroTik" },
          { key: "wg-quick", label: "Linux (wg-quick)" },
        ]}
      />
      <Typography.Paragraph type="secondary" style={{ marginBottom: 4 }}>
        {steps.intro}
      </Typography.Paragraph>
      <ol style={{ paddingLeft: 20, marginBottom: 12 }}>
        {steps.steps.map((step) => (
          <li key={step}>
            <Typography.Text>{step}</Typography.Text>
          </li>
        ))}
      </ol>

      <Typography.Paragraph copyable={{ text: peerConfig.data?.config ?? "" }}>
        <pre style={{ whiteSpace: "pre-wrap", margin: 0 }}>
          {peerConfig.isPending ? "Menyiapkan..." : peerConfig.data?.config}
        </pre>
      </Typography.Paragraph>

      <Typography.Paragraph type="secondary" style={{ marginBottom: 0 }}>
        {steps.verify}
      </Typography.Paragraph>
    </Modal>
  );
}
