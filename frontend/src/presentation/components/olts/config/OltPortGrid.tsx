import { Card, Empty, Space, Tag, Tooltip, Typography } from "antd";
import type { OltPort } from "@/domain/entities";
import { groupPortsBySlot } from "./oltSystem";

const { Text } = Typography;

function portColor(port: OltPort): string {
  if (!port.adminUp) return "default";
  return port.operUp ? "green" : "red";
}

function portState(port: OltPort): string {
  if (!port.adminUp) return "shutdown";
  return port.operUp ? "up" : "down";
}

interface OltPortGridProps {
  ports: OltPort[];
  kind: OltPort["kind"];
  emptyText: string;
  cardLabel: (slot: number) => string;
}

// One card per slot, its ports drawn as a grid. Colour carries the state an
// operator scans for: grey is administratively shut, red is a port that is
// enabled but has no light, green is carrying traffic.
export function OltPortGrid({
  ports,
  kind,
  emptyText,
  cardLabel,
}: OltPortGridProps) {
  const groups = groupPortsBySlot(ports, kind);

  if (groups.length === 0) {
    return <Empty description={emptyText} />;
  }

  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      {groups.map((group) => {
        const up = group.ports.filter((port) => port.operUp).length;
        return (
          <Card
            key={group.slot}
            size="small"
            title={cardLabel(group.slot)}
            extra={
              <Text type="secondary">
                {up} / {group.ports.length} up
              </Text>
            }
          >
            <Space wrap size={[8, 8]}>
              {group.ports.map((port) => (
                <Tooltip
                  key={port.ifIndex}
                  title={`${port.name} · ${portState(port)}`}
                >
                  <Tag
                    color={portColor(port)}
                    data-testid={`port-${port.name}`}
                    style={{ margin: 0, minWidth: 52, textAlign: "center" }}
                  >
                    {port.port}
                  </Tag>
                </Tooltip>
              ))}
            </Space>
          </Card>
        );
      })}
    </Space>
  );
}
