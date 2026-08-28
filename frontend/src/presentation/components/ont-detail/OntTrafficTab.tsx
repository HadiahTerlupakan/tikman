import {
  Alert,
  Card,
  Col,
  Descriptions,
  Empty,
  Row,
  Spin,
  Statistic,
  Tag,
  Typography,
} from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  SyncOutlined,
} from "@ant-design/icons";
import { formatBytes, formatRate } from "../trafficFormat";

const { Text } = Typography;

interface TrafficMetrics {
  rxMbps?: number | null;
  txMbps?: number | null;
  rxBytes?: number | null;
  txBytes?: number | null;
  rxPackets?: number | null;
  txPackets?: number | null;
}

interface OntTrafficTabProps {
  metrics?: TrafficMetrics;
  isLoading: boolean;
  isFetching: boolean;
  lastPolled: string;
}

// Rate and volume are what an operator opens this tab for, so they lead as
// figures rather than as rows in a list. Download is the OLT→ONU direction,
// which the OLT reports as TX.
export function OntTrafficTab({
  metrics,
  isLoading,
  isFetching,
  lastPolled,
}: OntTrafficTabProps) {
  if (isLoading) return <Spin />;
  if (!metrics) {
    return <Empty description="No traffic statistics available yet" />;
  }

  return (
    <>
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 12,
          marginBottom: 16,
        }}
      >
        <Tag
          icon={isFetching ? <SyncOutlined spin /> : undefined}
          color={isFetching ? "processing" : "success"}
          style={{ margin: 0 }}
        >
          {isFetching ? "Polling OLT…" : "Live from OLT"}
        </Tag>
        <Text type="secondary">Last polled: {lastPolled}</Text>
      </div>

      <Row gutter={[16, 16]}>
        <Col xs={12}>
          <Card size="small">
            <Statistic
              title="Download"
              value={formatRate(metrics.txMbps)}
              prefix={<ArrowDownOutlined style={{ color: "#52c41a" }} />}
              valueStyle={{ fontSize: 22 }}
            />
            <Text type="secondary">{formatBytes(metrics.txBytes)} total</Text>
          </Card>
        </Col>
        <Col xs={12}>
          <Card size="small">
            <Statistic
              title="Upload"
              value={formatRate(metrics.rxMbps)}
              prefix={<ArrowUpOutlined style={{ color: "#1677ff" }} />}
              valueStyle={{ fontSize: 22 }}
            />
            <Text type="secondary">{formatBytes(metrics.rxBytes)} total</Text>
          </Card>
        </Col>
      </Row>

      <Descriptions bordered column={1} size="small" style={{ marginTop: 16 }}>
        <Descriptions.Item label="Packets received (total)">
          {(metrics.rxPackets ?? 0).toLocaleString()}
        </Descriptions.Item>
        <Descriptions.Item label="Packets sent (total)">
          {(metrics.txPackets ?? 0).toLocaleString()}
        </Descriptions.Item>
      </Descriptions>

      <Alert
        type="info"
        showIcon
        style={{ marginTop: 16 }}
        message="Rates are read live every 15 seconds, which is how often the OLT recomputes them."
        description="The totals are the OLT's lifetime counters for this ONU, so usage over a period is the difference between two readings. Error counters are not shown: this OLT exposes no counter for them, and a zero would read as no errors rather than not measured."
      />
    </>
  );
}
