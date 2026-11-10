import { useEffect, useMemo } from "react";
import { Alert, Modal, Spin } from "antd";
import { useLibrenmsGraph } from "@/application/hooks";

export interface PreviewTarget {
  deviceId: number;
  ifName: string;
  label: string;
}

interface PortPreviewModalProps {
  target: PreviewTarget | null;
  onClose: () => void;
}

/** The same 24-hour PNG the group would receive, fetched through the API
 * client because the endpoint sits behind the session cookie. */
export function PortPreviewModal({ target, onClose }: PortPreviewModalProps) {
  const { data, isFetching, error } = useLibrenmsGraph(
    target?.deviceId,
    target?.ifName,
  );
  const url = useMemo(
    () => (data ? URL.createObjectURL(data) : undefined),
    [data],
  );

  useEffect(
    () => () => {
      if (url) URL.revokeObjectURL(url);
    },
    [url],
  );

  return (
    <Modal
      open={!!target}
      title={`Pratinjau — ${target?.label ?? ""}`}
      footer={null}
      width={960}
      onCancel={onClose}
      destroyOnHidden
    >
      {isFetching && (
        <div style={{ textAlign: "center", padding: 24 }}>
          <Spin />
        </div>
      )}
      {error && <Alert type="error" showIcon message={error.message} />}
      {url && !isFetching && (
        <img
          src={url}
          alt={`Grafik trafik ${target?.label ?? ""}`}
          style={{ width: "100%", height: "auto" }}
        />
      )}
    </Modal>
  );
}
