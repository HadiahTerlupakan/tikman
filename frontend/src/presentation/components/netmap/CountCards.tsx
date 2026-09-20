import { Card, Col, Row, Statistic } from "antd";
import type { MappingNode, NodeType, Olt } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

const ORDER: NodeType[] = ["server", "odc", "odp", "ont"];

// An OLT counts as on the map once it has both coordinates — the same test
// the backend's own sync (olt_map_node.go's syncOLTMapNode) uses to decide
// whether its mirror node exists at all, so this card and the map agree.
function placedOltCount(olts: Olt[]): number {
  return olts.filter(
    (olt) => olt.latitude !== undefined && olt.longitude !== undefined,
  ).length;
}

interface CountCardsProps {
  nodes: MappingNode[];
  olts: Olt[];
}

export function CountCards({ nodes, olts }: CountCardsProps) {
  const placed = placedOltCount(olts);

  return (
    <Row gutter={[12, 12]}>
      <Col xs={12} md={6}>
        <Card size="small">
          <Statistic
            title="OLT"
            value={`${placed} dari ${olts.length} terpasang`}
            valueStyle={{ color: NODE_COLORS.server }}
          />
          <span data-testid="count-olt-placed" hidden>
            {placed}
          </span>
        </Card>
      </Col>
      {ORDER.map((type) => {
        const count = nodes.filter((n) => n.type === type).length;
        return (
          <Col key={type} xs={12} md={6}>
            <Card size="small">
              <Statistic
                title={NODE_LABELS[type]}
                value={count}
                valueStyle={{ color: NODE_COLORS[type] }}
              />
              <span data-testid={`count-${type}`} hidden>
                {count}
              </span>
            </Card>
          </Col>
        );
      })}
    </Row>
  );
}
