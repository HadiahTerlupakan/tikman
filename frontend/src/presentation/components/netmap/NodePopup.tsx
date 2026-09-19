import { Button, Descriptions, Popconfirm, Space, Tag, Typography } from "antd";
import type { MappingNode } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

interface NodePopupProps {
  node: MappingNode;
  onEdit: (node: MappingNode) => void;
  onDelete: (nodeId: string) => void;
  onClose: () => void;
}

// A node exists to be placed first and described later, so most nodes will
// have most of these empty — an empty field renders nothing, never a "-".
function OptionalFields({ node }: { node: MappingNode }) {
  return (
    <Descriptions column={1} size="small">
      <Descriptions.Item label="Koordinat">
        {node.latitude.toFixed(6)}, {node.longitude.toFixed(6)}
      </Descriptions.Item>
      {node.capacity > 0 && (
        <Descriptions.Item label="Kapasitas">{node.capacity}</Descriptions.Item>
      )}
      {node.splitter && (
        <Descriptions.Item label="Splitter">{node.splitter}</Descriptions.Item>
      )}
      {node.pppoe && (
        <Descriptions.Item label="PPPoE">{node.pppoe}</Descriptions.Item>
      )}
      {node.serialNumber && (
        <Descriptions.Item label="Serial">
          {node.serialNumber}
        </Descriptions.Item>
      )}
      {node.notes && (
        <Descriptions.Item label="Catatan">{node.notes}</Descriptions.Item>
      )}
    </Descriptions>
  );
}

/**
 * The InfoWindow content for a marker: what a technician needs after tapping
 * a box on the map, plus the two actions that make sense from there. Reads
 * name first (what a person recognises), the code second (an identifier, not
 * a label), then only the fields actually filled in.
 */
export function NodePopup({ node, onEdit, onDelete, onClose }: NodePopupProps) {
  return (
    <Space
      direction="vertical"
      size={8}
      style={{ minWidth: 220, maxWidth: 280 }}
    >
      <div>
        <Typography.Title level={5} style={{ margin: 0 }}>
          {node.name}
        </Typography.Title>
        <Space size={6}>
          <Tag color={NODE_COLORS[node.type]}>{NODE_LABELS[node.type]}</Tag>
          <Typography.Text type="secondary">{node.nodeId}</Typography.Text>
        </Space>
      </div>
      <OptionalFields node={node} />
      <Space style={{ width: "100%", justifyContent: "flex-end" }}>
        <Button
          size="small"
          onClick={() => {
            onEdit(node);
            onClose();
          }}
        >
          Ubah
        </Button>
        <Popconfirm
          title="Hapus node ini?"
          okText="Ya"
          cancelText="Tidak"
          onConfirm={() => {
            onDelete(node.nodeId);
            onClose();
          }}
        >
          <Button size="small" danger>
            Hapus
          </Button>
        </Popconfirm>
      </Space>
    </Space>
  );
}
