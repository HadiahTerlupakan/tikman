import { Descriptions, Space, Typography } from "antd";
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

interface StatItem {
  label: string;
  value: string;
}

// "Kabel tergambar" (cables drawn) counts edges on the map; it is not the
// box's real occupancy, and must say so — see NodeNetworkContext.test.tsx's
// "different sources" test for why this label exists at all.
const DRAWN_LABEL = "Kabel tergambar";
const DRAWN_CASCADE_LABEL = "Kaskade tergambar";

function slotStats(info: NodeSlotInfo): StatItem[] {
  if (info.unlimited) {
    return [{ label: DRAWN_LABEL, value: "tanpa batas" }];
  }
  const ordinary = info.usages.find((usage) => !usage.cascade);
  const cascade = info.usages.find((usage) => usage.cascade);
  const stats: StatItem[] = ordinary
    ? [{ label: DRAWN_LABEL, value: `${ordinary.used} dari ${info.capacity}` }]
    : [];
  // Only shown once it is actually used: an unused cascade pool is not news,
  // but a nonzero one has to read as its own figure, never folded into the
  // row above — the trap this whole feature exists to get right.
  if (cascade && cascade.used > 0) {
    stats.push({
      label: DRAWN_CASCADE_LABEL,
      value: `${cascade.used} dari ${info.capacity}`,
    });
  }
  return stats;
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
 * box is, but what depends on it.
 *
 * The slot count and the subscriber count are never reconciled into one
 * figure or explained against each other — they come from different
 * sources (cables drawn on the map vs. real ONT registrations) and can
 * honestly disagree. That gap is the most useful thing on the screen: a
 * technician standing at the box needs to see it, not have it smoothed
 * away. Each row's label says which source it came from instead.
 *
 * Renders nothing at all when there is genuinely nothing to say (an
 * isolated node of a type capacity never gates, with no cables and no
 * subscriber count given).
 */
export function NodeNetworkContext({
  node,
  edges,
  nodesById,
  subscriberCount,
}: NodeNetworkContextProps) {
  const slots = slotUsage(node, edges, nodesById);
  const connections = connectionsLine(nodeConnections(node, edges, nodesById));
  const stats = slots ? slotStats(slots) : [];
  if (subscriberCount !== undefined) {
    stats.push({
      label: "Pelanggan terdaftar",
      value: `${subscriberCount} ONT`,
    });
  }

  if (stats.length === 0 && !connections) {
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
      {connections && <Typography.Text>{connections}</Typography.Text>}
      {stats.length > 0 && (
        <Descriptions column={1} size="small">
          {stats.map((stat) => (
            <Descriptions.Item key={stat.label} label={stat.label}>
              {stat.value}
            </Descriptions.Item>
          ))}
        </Descriptions>
      )}
    </Space>
  );
}
