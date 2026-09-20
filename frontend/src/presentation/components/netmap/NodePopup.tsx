import { Button, Descriptions, Space, Tag, Typography } from "antd";
import { useOdpSubscribers } from "@/application/hooks";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";
import { NodeDeleteControl } from "./NodeDeleteControl";
import { NodeNetworkContext } from "./NodeNetworkContext";

const ACTIONS_ROW_STYLE = {
  width: "100%",
  justifyContent: "flex-end" as const,
  borderTop: `1px solid ${colors.border}`,
  paddingTop: 8,
};

interface NodePopupProps {
  node: MappingNode;
  /** Every cable on the map and an index of every node by its nodeId — the
   * same `edges`/nodesById MapCanvas already builds to draw the map, reused
   * here for slot usage and connection counts rather than fetched again. */
  edges: MappingEdge[];
  nodesById: Map<string, MappingNode>;
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

// Ubah always stays: moving a mirror node is exactly how its position gets
// corrected, and UpdateNode already writes that back to the OLT record.
// Whether Hapus itself is offered is NodeDeleteControl's call, not this
// popup's — shared with NodeList so both surfaces refuse the same way.
function NodePopupActions({
  node,
  onEdit,
  onDelete,
  onClose,
}: {
  node: MappingNode;
  onEdit: (node: MappingNode) => void;
  onDelete: (nodeId: string) => void;
  onClose: () => void;
}) {
  return (
    <Space style={ACTIONS_ROW_STYLE} wrap>
      <Button
        size="small"
        onClick={() => {
          onEdit(node);
          onClose();
        }}
      >
        Ubah
      </Button>
      <NodeDeleteControl
        node={node}
        onDelete={(nodeId) => {
          onDelete(nodeId);
          onClose();
        }}
        onOltLinkClick={onClose}
      />
    </Space>
  );
}

/**
 * The InfoWindow content for a marker: what a technician needs after tapping
 * a box on the map, plus the two actions that make sense from there. Reads
 * name first (what a person recognises), the code second (an identifier, not
 * a label), then only the fields actually filled in.
 */
export function NodePopup({
  node,
  edges,
  nodesById,
  onEdit,
  onDelete,
  onClose,
}: NodePopupProps) {
  // A mapping node's own `id` is the real ODP id the subscriber list is
  // keyed on (see DistributionRepository.toOdp); `nodeId` is only its
  // fallback for a node that predates that column. Never asked for any
  // other type — a server or ONT has no ports to fetch subscribers for.
  const odpId = node.type === "odp" ? node.id ?? node.nodeId : undefined;
  const { data: subscribers } = useOdpSubscribers(odpId);

  return (
    <Space direction="vertical" size={8} className="netmap-popup">
      <div>
        <Typography.Title level={5} style={{ margin: 0 }}>
          {node.name}
        </Typography.Title>
        <Space size={6} style={{ marginTop: 4 }}>
          <Tag color={NODE_COLORS[node.type]}>{NODE_LABELS[node.type]}</Tag>
          <Typography.Text type="secondary" style={{ fontSize: 12 }}>
            {node.nodeId}
          </Typography.Text>
        </Space>
      </div>
      <OptionalFields node={node} />
      <NodeNetworkContext
        node={node}
        edges={edges}
        nodesById={nodesById}
        subscriberCount={subscribers?.length}
      />
      <NodePopupActions
        node={node}
        onEdit={onEdit}
        onDelete={onDelete}
        onClose={onClose}
      />
    </Space>
  );
}
