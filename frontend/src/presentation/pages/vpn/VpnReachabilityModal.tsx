import { useEffect, useState } from "react";
import { Alert, Button, Input, Modal, Typography } from "antd";
import { useTestReachability } from "@/application/hooks";

interface Props {
  peerId: string | null;
  siteName: string;
  subnets: string[];
  onClose: () => void;
}

export function VpnReachabilityModal({
  peerId,
  siteName,
  subnets,
  onClose,
}: Props) {
  const [address, setAddress] = useState("");
  const test = useTestReachability();
  const { reset } = test;

  useEffect(() => {
    // A result belongs to the address it was asked about. Carrying it into the
    // next peer would answer a question nobody asked.
    reset();
    setAddress("");
  }, [peerId, reset]);

  const run = () => {
    if (peerId && address.trim()) {
      test.mutate({ id: peerId, address: address.trim() });
    }
  };

  const result = test.data;

  return (
    <Modal
      open={!!peerId}
      title={`Uji koneksi ke perangkat di ${siteName}`}
      footer={null}
      onCancel={onClose}
    >
      <Typography.Paragraph type="secondary">
        Masukkan alamat IP perangkat di lokasi tersebut — OLT, router, atau apa
        pun yang menyala. Subnet yang dibawa tunnel ini:{" "}
        <Typography.Text code>
          {subnets.join(", ") || "belum ada"}
        </Typography.Text>
      </Typography.Paragraph>

      <Input.Search
        value={address}
        onChange={(event) => setAddress(event.target.value)}
        onSearch={run}
        placeholder="10.10.10.5"
        enterButton={
          <Button type="primary" loading={test.isPending}>
            Uji
          </Button>
        }
      />

      {test.isError && (
        <Alert
          style={{ marginTop: 16 }}
          type="error"
          showIcon
          message="Gagal menguji"
          description={(test.error as Error).message}
        />
      )}

      {result && (
        <Alert
          style={{ marginTop: 16 }}
          showIcon
          // Not routed is its own case: no packet was sent, so calling it a
          // failure would send the operator looking at the wrong end.
          type={
            result.reachable ? "success" : result.routed ? "error" : "warning"
          }
          message={
            result.reachable
              ? "Terjangkau"
              : result.routed
                ? "Tidak menjawab"
                : "Di luar subnet tunnel"
          }
          description={result.message}
        />
      )}
    </Modal>
  );
}
