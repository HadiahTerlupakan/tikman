import { Card, Table, Tooltip, Typography } from "antd";
import type {
  FlowSummary,
  FlowSummaryRow,
  FlowUpstreamShare,
} from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { asnLabel, peakWindowLabel } from "./routerFlows";
import { formatBps } from "./routerTraffic";

const SHARE_COLORS = ["#1677ff", "#52c41a", "#fa8c16", "#eb2f96", "#722ed1"];
const BAR_WIDTH = 160;
const BAR_HEIGHT = 10;

function UpstreamShareBar({
  shares,
  labels,
}: {
  shares: FlowUpstreamShare[];
  labels: Map<string, string>;
}) {
  const total = shares.reduce((sum, s) => sum + s.bytes, 0);
  if (total === 0) return null;
  return (
    <div style={{ display: "flex", width: BAR_WIDTH, height: BAR_HEIGHT }}>
      {shares.map((s, i) => (
        <Tooltip
          key={s.upstreamInterfaceId}
          title={`${labels.get(s.upstreamInterfaceId) ?? "?"}: ${formatBytes(s.bytes)}`}
        >
          <div
            style={{
              width: `${(s.bytes / total) * 100}%`,
              background: SHARE_COLORS[i % SHARE_COLORS.length],
            }}
          />
        </Tooltip>
      ))}
    </div>
  );
}

interface Props {
  data?: FlowSummary;
  labels: Map<string, string>;
  loading: boolean;
}

// The rows are the top 50 ASNs, so their percentages need not add up to 100:
// the total is shown on its own and the column is never summed.
export function FlowSummaryTable({ data, labels, loading }: Props) {
  const peakTitle = data
    ? `Puncak per ${peakWindowLabel(data.peakWindowSeconds)}`
    : "Puncak";
  return (
    <Card
      size="small"
      title="Tujuan"
      extra={
        data && (
          <Typography.Text>{`Total: ${formatBytes(data.totalBytes)}`}</Typography.Text>
        )
      }
    >
      <Table<FlowSummaryRow>
        size="small"
        rowKey="asn"
        loading={loading}
        pagination={false}
        dataSource={data?.rows ?? []}
        columns={[
          { title: "Organisasi", render: (_, r) => asnLabel(r.asn, r.name) },
          {
            title: "ASN",
            dataIndex: "asn",
            render: (asn: number) => asn || "—",
          },
          {
            title: "Total",
            dataIndex: "bytes",
            render: (b: number) => formatBytes(b),
          },
          {
            title: "%",
            dataIndex: "percent",
            render: (p: number) => `${p.toFixed(1)}%`,
          },
          {
            title: "Rata-rata",
            dataIndex: "avgBps",
            render: (v: number) => formatBps(v),
          },
          {
            title: peakTitle,
            dataIndex: "peakBps",
            render: (v: number) => formatBps(v),
          },
          {
            title: "Per upstream",
            render: (_, r) => (
              <UpstreamShareBar shares={r.upstreams} labels={labels} />
            ),
          },
        ]}
      />
    </Card>
  );
}
