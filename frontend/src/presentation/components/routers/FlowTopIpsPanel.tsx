import { Alert, Card, Table } from "antd";
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
        // DB-IP's free dataset is CC BY 4.0, which requires this credit.
        <a href="https://db-ip.com" target="_blank" rel="noopener noreferrer">
          Lokasi IP oleh DB-IP
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

/** Windows has no flag emoji and shows the two letters instead, still readable. */
function TopIpAddress({ row }: { row: FlowTopIp }) {
  const flag = countryFlag(row.country);
  if (!flag) return <>{row.ip}</>;
  return (
    <span>
      <span
        title={`Lokasi server: ${countryName(row.country)}`}
        style={{ marginRight: 8 }}
      >
        {flag}
      </span>
      {row.ip}
    </span>
  );
}
