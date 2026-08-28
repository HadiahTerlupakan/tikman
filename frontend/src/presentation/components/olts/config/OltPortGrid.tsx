import { Card, Empty, Space, Tooltip, Typography } from "antd";
import type { CardHealth, OltPort } from "@/domain/entities";
import { CardHealthBadges } from "./CardHealthBadges";
import { groupPortsBySlot } from "./oltSystem";

const { Text } = Typography;

type PortState = "up" | "down" | "shutdown";

const PORT_STATES: Record<PortState, { label: string; color: string }> = {
  up: { label: "Up", color: "#52c41a" },
  down: { label: "Down", color: "#ff4d4f" },
  shutdown: { label: "Shut down", color: "#8c8c8c" },
};

function portState(port: OltPort): PortState {
  if (!port.adminUp) return "shutdown";
  return port.operUp ? "up" : "down";
}

function PortBox({ port }: { port: OltPort }) {
  const state = portState(port);

  return (
    <Tooltip title={`${port.name} · ${PORT_STATES[state].label}`}>
      <div
        data-testid={`port-${port.name}`}
        style={{
          width: 46,
          height: 40,
          borderRadius: 6,
          border: `1px solid ${PORT_STATES[state].color}`,
          // Tinted rather than filled, so the number stays readable in both
          // the light and dark theme the app ships.
          background: `${PORT_STATES[state].color}22`,
          color: PORT_STATES[state].color,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
          fontVariantNumeric: "tabular-nums",
          fontWeight: 600,
        }}
      >
        {port.port}
      </div>
    </Tooltip>
  );
}

function PortLegend() {
  return (
    <Space size={16}>
      {(Object.keys(PORT_STATES) as PortState[]).map((state) => (
        <Space key={state} size={6}>
          <span
            style={{
              display: "inline-block",
              width: 12,
              height: 12,
              borderRadius: 3,
              border: `1px solid ${PORT_STATES[state].color}`,
              background: `${PORT_STATES[state].color}22`,
            }}
          />
          <Text type="secondary">{PORT_STATES[state].label}</Text>
        </Space>
      ))}
    </Space>
  );
}

interface OltPortGridProps {
  ports: OltPort[];
  kind: OltPort["kind"];
  emptyText: string;
  cardLabel: (slot: number) => string;
  cardHealth?: CardHealth[];
}

// One card per slot, its ports drawn as a grid. Colour carries the state an
// operator scans for: grey is administratively shut, red is a port that is
// enabled but has no light, green is carrying traffic.
export function OltPortGrid({
  ports,
  kind,
  emptyText,
  cardLabel,
  cardHealth = [],
}: OltPortGridProps) {
  const groups = groupPortsBySlot(ports, kind);

  if (groups.length === 0) {
    return <Empty description={emptyText} />;
  }

  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      <PortLegend />
      {groups.map((group) => {
        const up = group.ports.filter((port) => port.operUp).length;
        return (
          <Card
            key={group.slot}
            size="small"
            title={
              <Space size={12} wrap>
                <span>{cardLabel(group.slot)}</span>
                <CardHealthBadges
                  health={cardHealth.find(
                    (health) => health.slot === group.slot,
                  )}
                />
              </Space>
            }
            extra={
              <Text type="secondary">
                {up} / {group.ports.length} up
              </Text>
            }
          >
            <Space wrap size={[8, 8]}>
              {group.ports.map((port) => (
                <PortBox key={port.ifIndex} port={port} />
              ))}
            </Space>
          </Card>
        );
      })}
    </Space>
  );
}
