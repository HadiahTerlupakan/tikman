import { Button, Descriptions, Popconfirm, Space, Typography } from "antd";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { formatMeters, straightLineMeters } from "./cableMath";
import { DELETED_NODE_LABEL, FIBER_LABELS } from "./mappingLabels";

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

function Details({ edge }: { edge: MappingEdge }) {
  return (
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
  );
}

/**
 * How this cable was actually traced: its full drawn length against the
 * straight line between its ends, and how many corners it took to get
 * there. Zero bends with a drawn length close to the straight line usually
 * means the route was guessed, not walked — the length a technician should
 * not yet trust for ordering cable.
 */
function TraceInfo({
  edge,
  sourceNode,
  targetNode,
}: {
  edge: MappingEdge;
  sourceNode?: MappingNode;
  targetNode?: MappingNode;
}) {
  const bends = edge.waypoints?.length ?? 0;
  // Only the straight line is computed fresh here: the traced length is
  // already known (edge.distance, shown above as Panjang) and needs no
  // node coordinates, but there is no stored field to compare it against.
  const straight =
    sourceNode && targetNode
      ? straightLineMeters(sourceNode, targetNode)
      : undefined;
  const straightText =
    straight !== undefined ? ` (garis lurus ${formatMeters(straight)})` : "";
  const line = `${formatMeters(edge.distance)}${straightText} · ${bends} titik belok`;

  return (
    <Space
      direction="vertical"
      size={2}
      style={{
        width: "100%",
        borderTop: `1px solid ${colors.border}`,
        paddingTop: 8,
      }}
    >
      <Typography.Text
        type="secondary"
        style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: 0.5 }}
      >
        Jejak kabel
      </Typography.Text>
      <Typography.Text>{line}</Typography.Text>
    </Space>
  );
}

interface ActionsProps {
  edge: MappingEdge;
  onEdit: (edge: MappingEdge) => void;
  onRedraw: (edge: MappingEdge) => void;
  onDelete: (edgeId: string) => void;
  onClose: () => void;
}

function Actions({ edge, onEdit, onRedraw, onDelete, onClose }: ActionsProps) {
  return (
    <Space
      style={{
        width: "100%",
        justifyContent: "flex-end",
        borderTop: `1px solid ${colors.border}`,
        paddingTop: 8,
      }}
      wrap
    >
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
  );
}

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
    <Space direction="vertical" size={8} className="netmap-popup">
      <div>
        <Typography.Title level={5} style={{ margin: 0 }}>
          {sourceName} → {targetName}
        </Typography.Title>
        <div style={{ marginTop: 4 }}>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {edge.source} → {edge.target}
          </Typography.Text>
        </div>
      </div>
      <Details edge={edge} />
      <TraceInfo edge={edge} sourceNode={sourceNode} targetNode={targetNode} />
      <Actions
        edge={edge}
        onEdit={onEdit}
        onRedraw={onRedraw}
        onDelete={onDelete}
        onClose={onClose}
      />
    </Space>
  );
}
