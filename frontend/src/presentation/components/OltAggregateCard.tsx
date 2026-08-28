import { Card, Col, Empty, Row, Spin, Statistic, Typography } from "antd";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { useOltAggregateTraffic } from "@/application/hooks/useOlts";
import { percentile95 } from "./trafficStats";

const { Text } = Typography;

const DOWNLOAD_COLOUR = "#ff69b4";
const UPLOAD_COLOUR = "#4169e1";

function formatMbps(value?: number): string {
  if (value === undefined || !Number.isFinite(value)) return "—";
  if (value >= 1000) return `${(value / 1000).toFixed(2)} Gbps`;
  return `${value.toFixed(1)} Mbps`;
}

interface OltAggregateCardProps {
  oltId: string;
  oltName: string;
  period: string;
}

// The sum of every ONU under the OLT. A per-ONT graph cannot show whether the
// uplink or a PON is filling up; this is the figure that does.
export function OltAggregateCard({
  oltId,
  oltName,
  period,
}: OltAggregateCardProps) {
  const { data: points, isLoading } = useOltAggregateTraffic(oltId, period);

  if (isLoading) {
    return (
      <Card style={{ marginBottom: 16 }}>
        <Spin />
      </Card>
    );
  }
  if (!points || points.length === 0) {
    return (
      <Card style={{ marginBottom: 16 }}>
        <Empty description={`No aggregate traffic for ${oltName} yet`} />
      </Card>
    );
  }

  const chartData = points.map((point) => ({
    time: new Date(point.time).getTime(),
    download: point.txMbps,
    upload: point.rxMbps,
  }));
  const latest = points[points.length - 1];
  const peakDownload = Math.max(...points.map((p) => p.txMaxMbps));
  const p95Download = percentile95(points.map((p) => p.txMbps));

  return (
    <Card
      title={`${oltName} — total traffic`}
      style={{ marginBottom: 16 }}
      size="small"
    >
      <Row gutter={[16, 16]} style={{ marginBottom: 12 }}>
        <Col xs={12} md={6}>
          <Statistic
            title="Download now"
            value={formatMbps(latest.txMbps)}
            valueStyle={{ fontSize: 20, color: DOWNLOAD_COLOUR }}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title="Upload now"
            value={formatMbps(latest.rxMbps)}
            valueStyle={{ fontSize: 20, color: UPLOAD_COLOUR }}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title="Peak download"
            value={formatMbps(peakDownload)}
            valueStyle={{ fontSize: 20 }}
          />
        </Col>
        <Col xs={12} md={6}>
          <Statistic
            title="95th download"
            value={formatMbps(p95Download)}
            valueStyle={{ fontSize: 20 }}
          />
        </Col>
      </Row>

      <ResponsiveContainer width="100%" height={160}>
        <AreaChart data={chartData}>
          <CartesianGrid strokeDasharray="3 3" opacity={0.2} />
          <XAxis
            dataKey="time"
            type="number"
            domain={["dataMin", "dataMax"]}
            tick={{ fontSize: 10 }}
            tickFormatter={(value: number) =>
              new Date(value).toLocaleTimeString([], {
                hour: "2-digit",
                minute: "2-digit",
              })
            }
          />
          <YAxis tick={{ fontSize: 10 }} />
          <Tooltip
            labelFormatter={(label: unknown) =>
              typeof label === "number" ? new Date(label).toLocaleString() : ""
            }
            formatter={(value: unknown) =>
              formatMbps(typeof value === "number" ? value : undefined)
            }
          />
          <Area
            type="monotone"
            dataKey="download"
            stroke={DOWNLOAD_COLOUR}
            fill={DOWNLOAD_COLOUR}
            fillOpacity={0.2}
          />
          <Area
            type="monotone"
            dataKey="upload"
            stroke={UPLOAD_COLOUR}
            fill={UPLOAD_COLOUR}
            fillOpacity={0.2}
          />
        </AreaChart>
      </ResponsiveContainer>

      <Text type="secondary" style={{ fontSize: 11 }}>
        Summed across {latest.onlineOnts} ONUs reporting in the latest bucket.
      </Text>
    </Card>
  );
}
