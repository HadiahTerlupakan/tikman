import { Button, Descriptions, Popconfirm, Space, Typography } from "antd";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { formatMeters } from "./cableMath";
import { FIBER_LABELS } from "./mappingLabels";

interface EdgePopupProps {
  edge: MappingEdge;
  /** Resolved from the edge's `source`/`target` ids against the nodes on the
   * map — `undefined` when that node has since been deleted. Node deletion
   * never cascades to edges (migration 53's own design), so this is a real,
   * reachable state, not a defensive nicety. */
  sourceNode?: MappingNode;
  targetNode?: MappingNode;
  onEdit: (edge: MappingEdge) => void;
  onRedraw: (edge: MappingEdge) => void;
  onDelete: (edgeId: string) => void;
  onClose: () => void;
}

const DELETED_NODE_LABEL = "Node sudah dihapus";

/**
 * The InfoWindow content for a cable: its ends by name (an id alone sends a
 * technician to go look it up), what it is, how long it is, and the three
 * actions a cable on the map can take.
 */
export function EdgePopup({
  edge,
  sourceNode,
  targetNode,
  onEdit,
  onRedraw,
  onDelete,
  onClose,
}: EdgePopupProps) {
  const sourceName = sourceNode?.name ?? DELETED_NODE_LABEL;
  const targetName = targetNode?.name ?? DELETED_NODE_LABEL;

  return (
    <Space
      direction="vertical"
      size={8}
      style={{ minWidth: 220, maxWidth: 280 }}
    >
      <div>
        <Typography.Text strong>
          {sourceName} → {targetName}
        </Typography.Text>
        <div>
          <Typography.Text type="secondary">
            {edge.source} → {edge.target}
          </Typography.Text>
        </div>
      </div>
      <Descriptions column={1} size="small">
        <Descriptions.Item label="Jenis">
          {FIBER_LABELS[edge.fiberType]}
        </Descriptions.Item>
        <Descriptions.Item label="Panjang">
          {formatMeters(edge.distance)}
        </Descriptions.Item>
        {edge.notes && (
          <Descriptions.Item label="Catatan">{edge.notes}</Descriptions.Item>
        )}
      </Descriptions>
      <Space style={{ width: "100%", justifyContent: "flex-end" }} wrap>
        <Button
          size="small"
          onClick={() => {
            onEdit(edge);
            onClose();
          }}
        >
          Ubah
        </Button>
        <Button
          size="small"
          onClick={() => {
            onRedraw(edge);
            onClose();
          }}
        >
          Gambar ulang
        </Button>
        <Popconfirm
          title="Hapus kabel ini?"
          okText="Ya"
          cancelText="Tidak"
          onConfirm={() => {
            onDelete(edge.edgeId);
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
