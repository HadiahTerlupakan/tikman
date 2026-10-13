import { Alert, Card, Table, Tooltip } from "antd";
import type { FlowTopIp } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { asnLabel, countryFlag, countryName } from "./routerFlows";

const PAGE_SIZE = 20;

interface Props {
  available: boolean;
  rows: FlowTopIp[];
  loading: boolean;
  failed: boolean;
}

export function FlowTopIpsPanel({ available, rows, loading, failed }: Props) {
  return (
    <Card
      size="small"
      title="IP tujuan teratas"
      extra={
        // IPinfo Lite is CC BY-SA 4.0, which requires this credit.
        <a href="https://ipinfo.io" target="_blank" rel="noopener noreferrer">
          Data IP oleh IPinfo
        </a>
      }
    >
      {available && failed ? (
        <Alert type="error" showIcon message="Gagal memuat IP tujuan teratas" />
      ) : available ? (
        <Table<FlowTopIp>
          size="small"
          rowKey="ip"
          loading={loading}
          pagination={{ pageSize: PAGE_SIZE }}
          dataSource={rows}
          columns={[
            { title: "IP", render: (_, r) => <TopIpAddress row={r} /> },
            { title: "Organisasi", render: (_, r) => asnLabel(r.asn, r.name) },
            {
              title: "Total",
              dataIndex: "bytes",
              render: (b: number) => formatBytes(b),
            },
            {
              title: (
                <Tooltip title="Koneksi yang tertangkap sampling 1:100. Koneksi sebenarnya lebih banyak, terutama yang singkat.">
                  Koneksi
                </Tooltip>
              ),
              dataIndex: "flows",
              render: countOrDash,
            },
          ]}
        />
      ) : (
        <Alert
          type="info"
          showIcon
          message="IP tujuan hanya disimpan 7 hari. Pilih rentang di dalam 7 hari terakhir untuk melihatnya."
        />
      )}
    </Card>
  );
}

/** 0 means no window in the range recorded it yet, not "never reached". */
function countOrDash(n: number): string {
  return n > 0 ? n.toLocaleString("id-ID") : "—";
}

/** Windows has no flag emoji and shows the two letters instead, still readable. */
function TopIpAddress({ row }: { row: FlowTopIp }) {
  const flag = countryFlag(row.country);
  if (!flag) return <>{row.ip}</>;
  const fromAS = row.countrySource === "as";
  return (
    <span>
      <span
        title={
          fromAS
            ? `Negara terdaftar AS: ${countryName(row.country)} (lokasi server belum tercatat)`
            : `Lokasi server: ${countryName(row.country)}`
        }
        // Dimmed: an AS's registered country is often not where its server is.
        style={{ marginRight: 8, opacity: fromAS ? 0.45 : 1 }}
      >
        {flag}
      </span>
      {row.ip}
    </span>
  );
}
