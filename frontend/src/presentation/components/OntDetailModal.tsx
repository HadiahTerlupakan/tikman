import { Modal, Spin, Tag, Tabs, Descriptions } from "antd";
import { useState } from "react";
import { useOntMetrics } from "@/application/hooks/useOntMetrics";
import { ONTEventTimeline } from "./ONTEventTimeline";
import type { Ont } from "@/domain/entities";

interface OntDetailModalProps {
  ont: Ont;
  visible: boolean;
  onClose: () => void;
}

export function OntDetailModal({ ont, visible, onClose }: OntDetailModalProps) {
  const [activeTab, setActiveTab] = useState("basic");
  
  const isTrafficTab = activeTab === "traffic";
  const pollingInterval = isTrafficTab ? 3000 : 300000;
  
  const { data: metrics, isLoading } = useOntMetrics(
    ont.id, 
    ont.status === "online" && visible,
    pollingInterval
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
    getColor: (value: number) => string
  ) =>
    power === null || power === undefined ? (
      <Tag color="default">No signal</Tag>
    ) : (
      <Tag color={getColor(power)}>{power.toFixed(2)} dBm</Tag>
    );

  const formatBytes = (bytes: number | undefined) => {
    if (!bytes) return "0 B";
    const mb = bytes / 1024 / 1024;
    if (mb >= 1024) return `${(mb / 1024).toFixed(2)} GB`;
    if (mb >= 1) return `${mb.toFixed(2)} MB`;
    return `${(bytes / 1024).toFixed(2)} KB`;
  };

  const tabItems = [
    {
      key: "basic",
      label: "Basic Info",
      children: (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Serial Number">{ont.serialNumber}</Descriptions.Item>
          <Descriptions.Item label="OLT">{ont.oltName || ont.oltId}</Descriptions.Item>
          <Descriptions.Item label="Name">{ont.name || "-"}</Descriptions.Item>
          <Descriptions.Item label="Description">{ont.description || "-"}</Descriptions.Item>
          <Descriptions.Item label="Port ID">{ont.portId}</Descriptions.Item>
          <Descriptions.Item label="ONT ID">{ont.ontId}</Descriptions.Item>
          <Descriptions.Item label="Status">
            <Tag color={ont.status === "online" ? "success" : "default"}>
              {ont.status ? ont.status.toUpperCase() : 'UNKNOWN'}
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
            {(ont as any).deviceType || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Hardware Version">
            {(ont as any).hardwareVersion || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="Software Version">
            {(ont as any).softwareVersion || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="IP Address">
            {(ont as any).ipAddress || "-"}
          </Descriptions.Item>
          <Descriptions.Item label="MAC Address">
            {(ont as any).macAddress || "-"}
          </Descriptions.Item>
        </Descriptions>
      ),
    },
    {
      key: "optical",
      label: "Optical Metrics",
      children: isLoading ? (
        <Spin />
      ) : metrics || (ont as any).rxPower !== undefined ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="RX Power (ONU)">
            {renderPower((ont as any).rxPower || metrics?.rxPower, getRxColor)}
          </Descriptions.Item>
          <Descriptions.Item label="TX Power (ONU)">
            {renderPower((ont as any).txPower || metrics?.txPower, getTxColor)}
          </Descriptions.Item>
          <Descriptions.Item label="Distance">
            {(ont as any).distance || metrics?.distance || "-"} m
          </Descriptions.Item>
          {metrics?.time && (
            <Descriptions.Item label="Last Updated">
              {new Date(metrics.time).toLocaleString()}
            </Descriptions.Item>
          )}
        </Descriptions>
      ) : (
        <p style={{ color: "#999", padding: 16 }}>
          No optical metrics available yet. Metrics are collected every 5 minutes.
        </p>
      ),
    },
    {
      key: "health",
      label: "Health Monitoring",
      children: isLoading ? (
        <Spin />
      ) : metrics && (metrics.temperature || metrics.voltage || (ont as any).temperature) ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="Temperature">
            {(metrics.temperature || (ont as any).temperature || 0).toFixed(1)} °C
          </Descriptions.Item>
          <Descriptions.Item label="Voltage">
            {(metrics.voltage || (ont as any).voltage || 0).toFixed(2)} V
          </Descriptions.Item>
          <Descriptions.Item label="TX Bias Current">
            {((ont as any).txBiasCurrent || 0).toFixed(2)} mA
          </Descriptions.Item>
        </Descriptions>
      ) : (
        <p style={{ color: "#999", padding: 16 }}>
          No health monitoring data available yet.
        </p>
      ),
    },
    {
      key: "traffic",
      label: "Traffic Statistics",
      children: isLoading ? (
        <Spin />
      ) : metrics || (ont as any).rxBytes ? (
        <Descriptions bordered column={1} size="small">
          <Descriptions.Item label="RX Bytes">
            {formatBytes((ont as any).rxBytes || metrics?.rxBytes)}
          </Descriptions.Item>
          <Descriptions.Item label="TX Bytes">
            {formatBytes((ont as any).txBytes || metrics?.txBytes)}
          </Descriptions.Item>
          <Descriptions.Item label="RX Packets">
            {((ont as any).rxPackets || 0).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="TX Packets">
            {((ont as any).txPackets || 0).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="RX Errors">
            {((ont as any).rxErrors || 0).toLocaleString()}
          </Descriptions.Item>
          <Descriptions.Item label="TX Errors">
            {((ont as any).txErrors || 0).toLocaleString()}
          </Descriptions.Item>
        </Descriptions>
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
      <Tabs 
        defaultActiveKey="basic" 
        items={tabItems}
        onChange={setActiveTab}
      />
    </Modal>
  );
}
