import { Card, Table } from "antd";
import type { FlowPopRow } from "@/domain/entities";
import { formatBytes } from "../trafficFormat";
import { OTHER_POP, asnLabel, popLabel } from "./routerFlows";

interface Props {
  rows: FlowPopRow[];
  labels: Map<string, string>;
  loading: boolean;
  onSelect: (pop: string) => void;
}

export function FlowPopTable({ rows, labels, loading, onSelect }: Props) {
  return (
    <Card size="small" title="Per POP (klik baris untuk memfilter)">
      <Table<FlowPopRow>
        size="small"
        loading={loading}
        pagination={false}
        dataSource={rows}
        rowKey={(r) => r.popInterfaceId ?? OTHER_POP}
        onRow={(r) => ({
          onClick: () => onSelect(r.popInterfaceId ?? OTHER_POP),
          style: { cursor: "pointer" },
        })}
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
            title: "3 tujuan teratas",
            render: (_, r) =>
              r.topAsns.map((a) => asnLabel(a.asn, a.name)).join(", ") || "—",
          },
        ]}
      />
    </Card>
  );
}
