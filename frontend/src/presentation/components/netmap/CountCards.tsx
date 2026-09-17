import { Card, Col, Row, Statistic } from "antd";
import type { MappingNode, NodeType } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

const ORDER: NodeType[] = ["server", "odc", "odp", "ont"];

export function CountCards({ nodes }: { nodes: MappingNode[] }) {
  return (
    <Row gutter={[12, 12]}>
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
