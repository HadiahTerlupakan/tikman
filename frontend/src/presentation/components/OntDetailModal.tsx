import { Modal, Tag, Tabs, Descriptions } from "antd";
import { useMemo, useState } from "react";
import {
  useOntMetrics,
  useOntMetricsRealtime,
} from "@/application/hooks/useOntMetrics";
import { ONTEventTimeline } from "./ONTEventTimeline";
import type { Ont } from "@/domain/entities";
import { ontAddressLabel } from "./ontAddress";
import { ontStatusColor, ontStatusLabel } from "./ontStatus";
import { OntOpticalTab } from "./ont-detail/OntOpticalTab";
import { OntTrafficTab } from "./ont-detail/OntTrafficTab";

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
          <Descriptions.Item label="Address">
            {ontAddressLabel(ont.slot, ont.portId, ont.ontId)}
          </Descriptions.Item>
          <Descriptions.Item label="Status">
            <Tag color={ontStatusColor(ont.status)}>
              {ontStatusLabel(ont.status)}
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
      children: (
        <OntOpticalTab
          metrics={{
            rxPower: ont.rxPower ?? metrics?.rxPower,
            txPower: ont.txPower ?? metrics?.txPower,
            distance: ont.distance ?? metrics?.distance,
            time: metrics?.time,
          }}
          isLoading={isLoading}
        />
      ),
    },
    {
      key: "traffic",
      label: "Traffic Statistics",
      children: (
        <OntTrafficTab
          metrics={metrics}
          isLoading={isLoading}
          isFetching={isFetchingRealtime}
          lastPolled={lastPolled}
        />
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
