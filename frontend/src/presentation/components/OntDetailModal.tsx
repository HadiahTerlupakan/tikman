import { Modal, Spin, Tag, Tabs, Descriptions } from "antd";
import { useMemo, useState } from "react";
import {
  useOntMetrics,
  useOntMetricsRealtime,
} from "@/application/hooks/useOntMetrics";
import { ONTEventTimeline } from "./ONTEventTimeline";
import type { Ont } from "@/domain/entities";
import { formatBytes, formatRate } from "./trafficFormat";

interface OntWithMetrics extends Ont {
  metrics?: {
    rxPower?: number | null;
    txPower?: number | null;
    distance?: number | null;
  };
}

interface OntDetailModalProps {
  ont: OntWithMetrics;
  visible: boolean;
  onClose: () => void;
}

export function OntDetailModal({ ont, visible, onClose }: OntDetailModalProps) {
  const [activeTab, setActiveTab] = useState("basic");

  const isTrafficTab = activeTab === "traffic";

  const { data: dbMetrics, isLoading: isLoadingDb } = useOntMetrics(
    ont.id,
    ont.status === "online" && visible && !isTrafficTab,
    300000,
  );

  const {
    data: realtimeMetrics,
    isLoading: isLoadingRealtime,
    isFetching: isFetchingRealtime,
    dataUpdatedAt: realtimeUpdatedAt,
  } = useOntMetricsRealtime(
    ont.id,
    ont.status === "online" && visible && isTrafficTab,
  );

  const metrics = isTrafficTab ? realtimeMetrics : dbMetrics;
  const isLoading = isTrafficTab ? isLoadingRealtime : isLoadingDb;
  const lastPolled = useMemo(
    () =>
      realtimeUpdatedAt
        ? new Date(realtimeUpdatedAt).toLocaleTimeString()
        : "-",
    [realtimeUpdatedAt],
  );

  const getRxColor = (power: number) => {
    if (power >= -25) return "green";
    if (power >= -27) return "orange";
    return "red";
  };

  const getTxColor = (power: number) => {
    if (power >= 0 && power <= 4) return "green";
    return "red";
  };

  const renderPower = (
    power: number | null | undefined,
    getColor: (value: number) => string,
  ) =>
    power === null || power === undefined ? (
      <Tag color="default">No signal</Tag>
    ) : (
      <Tag color={getColor(power)}>{power.toFixed(2)} dBm</Tag>
    );

  const tabItems = [
    {
      key: "basic",
      label: "Basic Info",
      children: (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Serial Number">
            {ont.serialNumber}
          </Descriptions.Item>
          <Descriptions.Item label="OLT">
            {ont.oltName || ont.oltId}
          </Descriptions.Item>
          <Descriptions.Item label="Name">{ont.name || "-"}</Descriptions.Item>
          <Descriptions.Item label="Description">
            {ont.description || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Port ID">{ont.portId}</Descriptions.Item>
          <Descriptions.Item label="ONT ID">{ont.ontId}</Descriptions.Item>
          <Descriptions.Item label="Status">
            <Tag color={ont.status === "online" ? "success" : "default"}>
              {ont.status ? ont.status.toUpperCase() : "UNKNOWN"}
            </Tag>
          </Descriptions.Item>
          <Descriptions.Item label="Last Seen">
            {ont.lastSeenAt ? new Date(ont.lastSeenAt).toLocaleString() : "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Created">
            {new Date(ont.createdAt).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="Updated">
            {new Date(ont.updatedAt).toLocaleString()}
          </Descriptions.Item>
        </Descriptions>
      ),
    },
    {
      key: "device",
      label: "Device Info",
      children: (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Device Type">
            {ont.deviceType || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Hardware Version">
            {ont.hardwareVersion || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Software Version">
            {ont.softwareVersion || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="IP Address">
            {ont.ipAddress || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="MAC Address">
            {ont.macAddress || "-"}
          </Descriptions.Item>
        </Descriptions>
      ),
    },
    {
      key: "optical",
      label: "Optical Metrics",
      children: isLoading ? (
        <Spin />
      ) : metrics || ont.rxPower !== undefined ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="RX Power (ONU)">
            {renderPower(ont.rxPower || metrics?.rxPower, getRxColor)}
          </Descriptions.Item>
          <Descriptions.Item label="TX Power (ONU)">
            {renderPower(ont.txPower || metrics?.txPower, getTxColor)}
          </Descriptions.Item>
          <Descriptions.Item label="Distance">
            {ont.distance || metrics?.distance || "-"} m
          </Descriptions.Item>
          {metrics?.time && (
            <Descriptions.Item label="Last Updated">
              {new Date(metrics.time).toLocaleString()}
            </Descriptions.Item>
          )}
        </Descriptions>
      ) : (
        <p style={{ color: "#999", padding: 16 }}>
          No optical metrics available yet. Metrics are collected every 5
          minutes.
        </p>
      ),
    },
    {
      key: "traffic",
      label: "Traffic Statistics",
      children: isLoading ? (
        <Spin />
      ) : metrics ? (
        <div>
          <div
            style={{
              display: "flex",
              alignItems: "center",
              justifyContent: "space-between",
              gap: 12,
              marginBottom: 12,
              padding: "10px 12px",
              border: "1px solid rgba(22, 119, 255, 0.25)",
              borderRadius: 8,
              background: "rgba(22, 119, 255, 0.08)",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span
                style={{
                  width: 8,
                  height: 8,
                  borderRadius: "50%",
                  background: isFetchingRealtime ? "#faad14" : "#52c41a",
                  boxShadow: `0 0 0 4px ${isFetchingRealtime ? "rgba(250, 173, 20, 0.18)" : "rgba(82, 196, 26, 0.18)"}`,
                }}
              />
              <Tag color="processing" style={{ margin: 0 }}>
                Live SNMP
              </Tag>
              <span style={{ color: "#8c8c8c", fontSize: 12 }}>
                {isFetchingRealtime ? "Polling OLT…" : "OLT replied"}
              </span>
            </div>
            <span style={{ color: "#8c8c8c", fontSize: 12 }}>
              Last polled: {lastPolled}
            </span>
          </div>

          <Descriptions bordered column={1} size="small">
            <Descriptions.Item label="Download Rate">
              {formatRate(metrics.txMbps)}
            </Descriptions.Item>
            <Descriptions.Item label="Upload Rate">
              {formatRate(metrics.rxMbps)}
            </Descriptions.Item>
            <Descriptions.Item label="Downloaded (total)">
              {formatBytes(metrics.txBytes)}
            </Descriptions.Item>
            <Descriptions.Item label="Uploaded (total)">
              {formatBytes(metrics.rxBytes)}
            </Descriptions.Item>
            <Descriptions.Item label="Packets received (total)">
              {(metrics.rxPackets ?? 0).toLocaleString()}
            </Descriptions.Item>
            <Descriptions.Item label="Packets sent (total)">
              {(metrics.txPackets ?? 0).toLocaleString()}
            </Descriptions.Item>
          </Descriptions>
          <div style={{ marginTop: 10, color: "#8c8c8c", fontSize: 12 }}>
            Rates are queried live from the OLT every 3 seconds. The totals are
            the OLT's lifetime counters for this ONU, so usage over a period is
            the difference between two readings. Error counters are not shown:
            this OLT exposes no counter for them, and a zero would read as "no
            errors" rather than "not measured".
          </div>
        </div>
      ) : (
        <p style={{ color: "#999", padding: 16 }}>
          No traffic statistics available yet.
        </p>
      ),
    },
    {
      key: "events",
      label: "Events & Availability",
      children: <ONTEventTimeline ontId={ont.id} days={7} />,
    },
  ];

  return (
    <Modal
      title={`ONT Details - ${ont.serialNumber}`}
      open={visible}
      onCancel={onClose}
      footer={null}
      width={800}
    >
      <Tabs defaultActiveKey="basic" items={tabItems} onChange={setActiveTab} />
    </Modal>
  );
}
