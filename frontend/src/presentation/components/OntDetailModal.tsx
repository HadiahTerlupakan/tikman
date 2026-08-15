import { Modal, Spin, Divider, Tag } from "antd";
import { useOntMetrics } from "@/application/hooks/useOntMetrics";
import type { Ont } from "@/domain/entities";

interface OntDetailModalProps {
  ont: Ont;
  visible: boolean;
  onClose: () => void;
}

export function OntDetailModal({ ont, visible, onClose }: OntDetailModalProps) {
  const { data: metrics, isLoading } = useOntMetrics(ont.id, ont.status === "online" && visible);

  const getRxColor = (power: number) => {
    if (power >= -25) return "green";
    if (power >= -27) return "orange";
    return "red";
  };

  const getTxColor = (power: number) => {
    if (power >= 0 && power <= 4) return "green";
    return "red";
  };

  return (
    <Modal
      title="ONT Details"
      open={visible}
      onCancel={onClose}
      footer={null}
      width={700}
    >
      <div>
        <p><strong>Serial Number:</strong> {ont.serialNumber}</p>
        <p><strong>OLT:</strong> {ont.oltName || ont.oltId}</p>
        <p><strong>Port ID:</strong> {ont.portId}</p>
        <p><strong>ONT ID:</strong> {ont.ontId}</p>
        <p><strong>Status:</strong> <Tag color={ont.status === "online" ? "success" : "default"}>{ont.status.toUpperCase()}</Tag></p>
        <p><strong>Description:</strong> {ont.description || "-"}</p>
        <p><strong>Last Seen:</strong> {ont.lastSeenAt ? new Date(ont.lastSeenAt).toLocaleString() : "-"}</p>
        <p><strong>Created:</strong> {new Date(ont.createdAt).toLocaleString()}</p>
        <p><strong>Updated:</strong> {new Date(ont.updatedAt).toLocaleString()}</p>

        {ont.status === "online" && (
          <>
            <Divider />
            <h4>Signal Metrics</h4>
            {isLoading ? (
              <Spin />
            ) : metrics ? (
              <>
                <p><strong>Rx Power:</strong> <Tag color={getRxColor(metrics.rxPower)}>{metrics.rxPower.toFixed(2)} dBm</Tag></p>
                <p><strong>Tx Power:</strong> <Tag color={getTxColor(metrics.txPower)}>{metrics.txPower.toFixed(2)} dBm</Tag></p>
                <p><strong>Temperature:</strong> {metrics.temperature.toFixed(1)} °C</p>
                <p><strong>Voltage:</strong> {metrics.voltage.toFixed(2)} V</p>
                <p><strong>Distance:</strong> {metrics.distance} m</p>
                <p><strong>Traffic (Rx/Tx):</strong> {(metrics.rxBytes / 1024 / 1024).toFixed(2)} MB / {(metrics.txBytes / 1024 / 1024).toFixed(2)} MB</p>
                <p><strong>Last Updated:</strong> {new Date(metrics.time).toLocaleString()}</p>
              </>
            ) : (
              <p style={{ color: "#999" }}>No metrics data available yet. Metrics are collected every 5 minutes.</p>
            )}
          </>
        )}
      </div>
    </Modal>
  );
}
