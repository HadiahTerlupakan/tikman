import { Card, Col, Row, Statistic, Tag, Typography } from "antd";
import type { Ont, OltSystemSnapshot } from "@/domain/entities";
import { OntStatus } from "@/domain/entities";
import { formatUptime, isPowerEntity, summariseModel } from "./oltSystem";
import { ontStatusLabel } from "../../ontStatus";

const { Text } = Typography;

const SUMMARY_STATUSES = [
  OntStatus.ONLINE,
  OntStatus.LOS,
  OntStatus.DYING_GASP,
  OntStatus.OFFLINE,
];

interface OltConfigHeaderProps {
  snapshot?: OltSystemSnapshot;
  onts: Ont[];
  totalOnts: number;
}

// The chassis summary, drawn from the SNMP snapshot the poll cached. Every
// figure here is read-only: nothing on this page writes to the OLT.
export function OltConfigHeader({
  snapshot,
  onts,
  totalOnts,
}: OltConfigHeaderProps) {
  const system = snapshot?.system;
  const ports = snapshot?.ports ?? [];
  const entities = system?.entities ?? [];

  const ponSlots = new Set(
    ports.filter((port) => port.kind === "pon").map((port) => port.slot),
  );
  const uplinkSlots = new Set(
    ports.filter((port) => port.kind === "uplink").map((port) => port.slot),
  );

  return (
    <Card size="small" style={{ marginBottom: 16 }}>
      <Row gutter={[16, 16]}>
        <Col xs={12} md={6}>
          <Statistic
            title="Model"
            value={system ? summariseModel(system.description) : "—"}
            valueStyle={{ fontSize: 18 }}
          />
          <Text type="secondary">{system?.name || "—"}</Text>
        </Col>
        <Col xs={12} md={5}>
          <Statistic
            title="Uptime"
            value={formatUptime(system?.uptimeSeconds ?? 0)}
            valueStyle={{ fontSize: 18 }}
          />
        </Col>
        <Col xs={12} md={4}>
          <Statistic title="PON cards" value={ponSlots.size} />
        </Col>
        <Col xs={12} md={4}>
          <Statistic title="Uplink cards" value={uplinkSlots.size} />
        </Col>
        <Col xs={12} md={5}>
          <Statistic
            title="Power modules"
            value={entities.filter(isPowerEntity).length}
          />
        </Col>
      </Row>

      <Row gutter={[16, 16]} style={{ marginTop: 16 }}>
        <Col xs={12} md={5}>
          <Statistic title="Total ONU" value={totalOnts} />
        </Col>
        <Col xs={24} md={19}>
          <Text type="secondary">
            ONU state
            {onts.length < totalOnts &&
              ` (first ${onts.length} of ${totalOnts})`}
          </Text>
          <div style={{ marginTop: 8 }}>
            {SUMMARY_STATUSES.map((status) => (
              <Tag key={status} data-testid={`onu-count-${status}`}>
                {ontStatusLabel(status)}:{" "}
                {onts.filter((ont) => ont.status === status).length}
              </Tag>
            ))}
          </div>
        </Col>
      </Row>
    </Card>
  );
}
