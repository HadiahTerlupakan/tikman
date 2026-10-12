import { Alert, Card, Table } from "antd";
import type { FlowTopIp } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { asnLabel } from "./routerFlows";

const PAGE_SIZE = 20;

interface Props {
  available: boolean;
  rows: FlowTopIp[];
  loading: boolean;
}

export function FlowTopIpsPanel({ available, rows, loading }: Props) {
  return (
    <Card size="small" title="IP tujuan teratas">
      {available ? (
        <Table<FlowTopIp>
          size="small"
          rowKey="ip"
          loading={loading}
          pagination={{ pageSize: PAGE_SIZE }}
          dataSource={rows}
          columns={[
            { title: "IP", dataIndex: "ip" },
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
