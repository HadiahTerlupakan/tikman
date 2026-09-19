import { Space, Typography } from "antd";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { colors } from "@/shared/theme";
import { DELETED_NODE_LABEL, FIBER_LABELS, NODE_LABELS } from "./mappingLabels";
import { nodeConnections, type NodeConnections } from "./nodeConnections";
import { slotUsage, type NodeSlotInfo } from "./slotUsage";

interface NodeNetworkContextProps {
  node: MappingNode;
  edges: MappingEdge[];
  nodesById: Map<string, MappingNode>;
  /** Real ONT assignments on this box, from the ODP subscriber list —
   * NodePopup is the one that fetches it, and only for a node of type odp,
   * so this stays undefined for every other type. */
  subscriberCount?: number;
}

function slotLines(info: NodeSlotInfo): string[] {
  if (info.unlimited) {
    return ["tanpa batas"];
  }
  const ordinary = info.usages.find((usage) => !usage.cascade);
  const cascade = info.usages.find((usage) => usage.cascade);
  const lines = ordinary
    ? [`${ordinary.used} dari ${info.capacity} terpakai`]
    : [];
  // Only mentioned once it is actually used: an unused cascade pool is not
  // news, but a nonzero one has to read as its own figure, never folded into
  // the line above.
  if (cascade && cascade.used > 0) {
    lines.push(
      `${cascade.used} dari ${info.capacity} untuk kaskade ke ${NODE_LABELS[cascade.targetType]}`,
    );
  }
  return lines;
}

function connectionsLine(connections: NodeConnections): string | undefined {
  const from = connections.incoming
    .map(
      (c) =>
        `${c.node?.name ?? DELETED_NODE_LABEL} (${FIBER_LABELS[c.fiberType]})`,
    )
    .join(", ");
  const to = connections.outgoing
    .map((c) => `${c.count} ${NODE_LABELS[c.type]}`)
    .join(", ");
  const parts = [from && `Dari: ${from}`, to && `Ke: ${to}`].filter(Boolean);
  return parts.length > 0 ? parts.join(" · ") : undefined;
}

/**
 * Where this node sits in the network: how full it is, what feeds it and
 * what hangs off it, and — for an ODP — how many ONTs are really assigned
 * to it. Kept in its own visually distinct block from the node's own
 * attributes above, since it answers a different question: not what this
 * box is, but what depends on it. Renders nothing at all when there is
 * genuinely nothing to say (an isolated node of a type capacity never
 * gates, with no cables and no subscriber count given).
 */
export function NodeNetworkContext({
  node,
  edges,
  nodesById,
  subscriberCount,
}: NodeNetworkContextProps) {
  const slots = slotUsage(node, edges, nodesById);
  const connections = connectionsLine(nodeConnections(node, edges, nodesById));
  const hasSubscribers = subscriberCount !== undefined;

  if (!slots && !connections && !hasSubscribers) {
    return null;
  }

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
        Posisi jaringan
      </Typography.Text>
      {slots &&
        slotLines(slots).map((line) => (
          <Typography.Text key={line}>{line}</Typography.Text>
        ))}
      {connections && <Typography.Text>{connections}</Typography.Text>}
      {hasSubscribers && (
        <Typography.Text>{`Pelanggan: ${subscriberCount} ONT`}</Typography.Text>
      )}
    </Space>
  );
}
