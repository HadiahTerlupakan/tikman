import { Alert, Drawer, Space, Table, Typography } from "antd";
import type {
  FlowPeering,
  FlowPeeringPop,
  FlowPeeringRow,
} from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { UpstreamShareBar } from "./FlowSummaryTable";
import { TrafficChart } from "./TrafficChart";
import {
  OTHER_POP,
  asnLabel,
  peakWindowLabel,
  peeringChartRows,
  popLabel,
} from "./routerFlows";
import { formatBps } from "./routerTraffic";

// Per POP and minute only the 20 largest destinations keep their own rows, so
// a destination that is not always among them is partly counted in Lainnya.
const FOLD_NOTE =
  "Tujuan yang tidak selalu masuk 20 besar per POP per menit sebagian tercatat sebagai Lainnya, jadi angkanya bisa sedikit lebih kecil dari sebenarnya.";

interface Props {
  asn: number;
  name: string;
  data?: FlowPeering;
  loading: boolean;
  failed: boolean;
  labels: Map<string, string>;
  onClose: () => void;
}

export function FlowPeeringDrawer({
  asn,
  name,
  data,
  loading,
  failed,
  labels,
  onClose,
}: Props) {
  const chart = peeringChartRows(data, labels);
  const peakTitle = data
    ? `Puncak per ${peakWindowLabel(data.peakWindowSeconds)}`
    : "Puncak";
  return (
    <Drawer
      open
      width={760}
      title={`${asnLabel(asn, name)} — per peering`}
      onClose={onClose}
    >
      {failed ? (
        <Alert type="error" showIcon message="Gagal memuat data per peering" />
      ) : (
        <Space direction="vertical" size="large" style={{ width: "100%" }}>
          {data && (
            <Typography.Text>{`Total: ${formatBytes(data.totalBytes)}`}</Typography.Text>
          )}
          <Table<FlowPeeringRow>
            size="small"
            rowKey="upstreamInterfaceId"
            loading={loading}
            pagination={false}
            dataSource={data?.rows ?? []}
            columns={[
              {
                title: "Peering / upstream",
                render: (_, r) =>
                  labels.get(r.upstreamInterfaceId) ?? r.upstreamInterfaceId,
              },
              {
                title: "Total",
                dataIndex: "bytes",
                render: (b: number) => formatBytes(b),
              },
              {
                title: "% dari tujuan ini",
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
            ]}
          />
          <TrafficChart
            stacked
            title="Per peering (bit/s)"
            series={chart.series}
            data={chart.rows}
          />
          <Typography.Title level={5} style={{ margin: 0 }}>
            Per POP (VLAN)
          </Typography.Title>
          <Table<FlowPeeringPop>
            size="small"
            rowKey={(r) => r.popInterfaceId ?? OTHER_POP}
            loading={loading}
            pagination={false}
            dataSource={data?.pops ?? []}
            columns={[
              {
                title: "POP",
                render: (_, r) => popLabel(r.popInterfaceId, labels),
              },
              {
                title: "Total",
                dataIndex: "bytes",
                render: (b: number) => formatBytes(b),
              },
              {
                title: "% dari tujuan ini",
                dataIndex: "percent",
                render: (p: number) => `${p.toFixed(1)}%`,
              },
              {
                title: "Per peering",
                render: (_, r) => (
                  <UpstreamShareBar shares={r.upstreams} labels={labels} />
                ),
              },
            ]}
          />
          {asn !== 0 && <Alert type="info" showIcon message={FOLD_NOTE} />}
        </Space>
      )}
    </Drawer>
  );
}
