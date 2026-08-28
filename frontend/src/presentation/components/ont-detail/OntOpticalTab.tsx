import { Card, Col, Empty, Row, Spin, Statistic, Tag, Typography } from "antd";
import { rxSignalQuality, txSignalQuality } from "./signalQuality";

const { Text } = Typography;

interface OpticalMetrics {
  rxPower?: number | null;
  txPower?: number | null;
  distance?: number | null;
  time?: string;
}

interface OntOpticalTabProps {
  metrics?: OpticalMetrics;
  isLoading: boolean;
}

function PowerCard({
  title,
  power,
  quality,
}: {
  title: string;
  power?: number | null;
  quality: (power: number) => { label: string; color: string };
}) {
  if (power === null || power === undefined) {
    return (
      <Card size="small">
        <Statistic
          title={title}
          value="No signal"
          valueStyle={{ fontSize: 22 }}
        />
      </Card>
    );
  }

  const { label, color } = quality(power);

  return (
    <Card size="small">
      <Statistic
        title={title}
        value={power.toFixed(2)}
        suffix="dBm"
        valueStyle={{ fontSize: 22 }}
      />
      <Tag color={color} style={{ marginTop: 4 }}>
        {label}
      </Tag>
    </Card>
  );
}

// Power readings lead as figures with a plain-language verdict beside them: a
// colour alone leaves the operator to remember where the thresholds sit.
export function OntOpticalTab({ metrics, isLoading }: OntOpticalTabProps) {
  if (isLoading) return <Spin />;
  if (!metrics) {
    return (
      <Empty description="No optical metrics yet. The poll collects them every minute." />
    );
  }

  return (
    <>
      <Row gutter={[16, 16]}>
        <Col xs={12}>
          <PowerCard
            title="RX power (ONU receive)"
            power={metrics.rxPower}
            quality={rxSignalQuality}
          />
        </Col>
        <Col xs={12}>
          <PowerCard
            title="TX power (ONU transmit)"
            power={metrics.txPower}
            quality={txSignalQuality}
          />
        </Col>
        <Col xs={12}>
          <Card size="small">
            <Statistic
              title="Distance"
              value={metrics.distance ?? "—"}
              suffix={metrics.distance ? "m" : undefined}
              valueStyle={{ fontSize: 22 }}
            />
          </Card>
        </Col>
      </Row>
      {metrics.time && (
        <Text type="secondary" style={{ display: "block", marginTop: 12 }}>
          Last updated: {new Date(metrics.time).toLocaleString()}
        </Text>
      )}
    </>
  );
}
