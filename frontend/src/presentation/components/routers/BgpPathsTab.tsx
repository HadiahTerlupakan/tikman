import { useMemo, useState } from "react";
import {
  Alert,
  Button,
  Card,
  Space,
  Spin,
  Switch,
  Table,
  Tag,
  Typography,
} from "antd";
import type { RouteAdvice, RoutePath, TrafficRange } from "@/domain/entities";
import { useFlowRouteAdvice } from "@/application/hooks/useRouterFlows";
import { useAsNames } from "@/application/hooks/useRouterPaths";
import { formatBytes } from "../trafficFormat";
import { AsPathDiagram } from "./AsPathDiagram";
import { buildPathGraph } from "./asPathGraph";
import { RouteLookupPanel } from "./RouteLookupPanel";
import { adviceNames, advicePaths, graphAsns } from "./bgpPaths";

// The advice is built from the top-IP table, which the server keeps 7 days.
const OVERVIEW_RANGE: TrafficRange = { kind: "relative", hours: 7 * 24 };
const LEGEND =
  "Garis penuh: jalur yang dipilih router. Putus-putus: alternatif yang diterima tapi tidak dipilih. Tebal garis mengikuti trafik 7 hari. Klik AS tujuan untuk melihat prefix-prefixnya.";

function Candidates({ row }: { row: RouteAdvice }) {
  const all: Array<RoutePath & { chosen: boolean }> = [
    ...(row.active ? [{ ...row.active, chosen: true }] : []),
    ...(row.alternatives ?? []).map((p) => ({ ...p, chosen: false })),
  ];
  return (
    <Space direction="vertical" size={2}>
      {all.map((p) => (
        <span key={`${p.peer}|${p.asPath}`}>
          {p.chosen ? <Tag color="green">Aktif</Tag> : <Tag>Alternatif</Tag>}
          {p.peer} — <Typography.Text code>{p.asPath}</Typography.Text> (lp{" "}
          {p.localPref})
        </span>
      ))}
    </Space>
  );
}

const detailColumns = [
  { title: "Prefix", dataIndex: "prefix" },
  {
    title: "Download",
    dataIndex: "downloadBytes",
    render: (b: number) => formatBytes(b),
  },
  {
    title: "Kandidat",
    key: "candidates",
    render: (_: unknown, row: RouteAdvice) => <Candidates row={row} />,
  },
];

interface Props {
  routerId: string;
  routerName: string;
}

export function BgpPathsTab({ routerId, routerName }: Props) {
  const [withAlternatives, setWithAlternatives] = useState(false);
  const [selectedAsn, setSelectedAsn] = useState<number | null>(null);
  const advice = useFlowRouteAdvice(routerId, OVERVIEW_RANGE, {}, true);
  const rows = useMemo(() => advice.data ?? [], [advice.data]);
  const graph = useMemo(
    () => buildPathGraph(advicePaths(rows, withAlternatives)),
    [rows, withAlternatives],
  );
  const names = useAsNames(routerId, graphAsns(graph));
  const labels = { ...adviceNames(rows), ...(names.data ?? {}) };
  const selectedRows = rows.filter((r) => r.asn === selectedAsn);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <Card
        size="small"
        title="Jalur AS ke tujuan tersibuk (7 hari)"
        extra={
          <Space>
            <Typography.Text type="secondary">
              Tampilkan alternatif
            </Typography.Text>
            <Switch
              aria-label="Tampilkan alternatif"
              checked={withAlternatives}
              onChange={setWithAlternatives}
            />
          </Space>
        }
      >
        <div data-testid="path-overview">
          {advice.isError ? (
            <Alert type="error" showIcon message="Gagal memuat jalur BGP" />
          ) : advice.isLoading ? (
            <Spin />
          ) : (
            <Space direction="vertical" size="middle" style={{ width: "100%" }}>
              <AsPathDiagram
                graph={graph}
                names={labels}
                routerLabel={routerName}
                selectedAsn={selectedAsn}
                onSelect={(n) => setSelectedAsn(n.asn)}
              />
              <Typography.Text type="secondary">{LEGEND}</Typography.Text>
            </Space>
          )}
        </div>
        {selectedAsn !== null && (
          <div data-testid="path-detail" style={{ marginTop: 16 }}>
            <Space style={{ marginBottom: 8 }}>
              <Typography.Text strong>
                AS{selectedAsn} {labels[String(selectedAsn)] ?? ""}
              </Typography.Text>
              <Button size="small" onClick={() => setSelectedAsn(null)}>
                Tutup
              </Button>
            </Space>
            <Table<RouteAdvice>
              size="small"
              rowKey="prefix"
              columns={detailColumns}
              dataSource={selectedRows}
              pagination={false}
            />
          </div>
        )}
      </Card>
      <RouteLookupPanel routerId={routerId} routerName={routerName} />
    </Space>
  );
}
