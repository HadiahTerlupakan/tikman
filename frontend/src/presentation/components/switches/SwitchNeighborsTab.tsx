import { useState } from "react";
import { Card, Input, Space, Table } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { SwitchNeighbor } from "@/domain/entities";
import { useSwitchNeighbors } from "@/application/hooks/useSwitches";

const lldpColumns: ColumnsType<SwitchNeighbor> = [
  { title: "Port", dataIndex: "port" },
  { title: "Perangkat", dataIndex: "remoteName" },
  { title: "Port perangkat", dataIndex: "remotePort" },
  { title: "Platform", dataIndex: "remotePlatform" },
];

const macColumns: ColumnsType<SwitchNeighbor> = [
  { title: "MAC", dataIndex: "mac" },
  { title: "VLAN", render: (_, n) => n.vid ?? "—" },
  { title: "Port", dataIndex: "port" },
];

function search(rows: SwitchNeighbor[], text: string): SwitchNeighbor[] {
  const needle = text.toLowerCase();
  return rows.filter((n) =>
    Object.values(n).join(" ").toLowerCase().includes(needle),
  );
}

function NeighborTable(props: {
  title: string;
  placeholder: string;
  rows: SwitchNeighbor[];
  columns: ColumnsType<SwitchNeighbor>;
  loading: boolean;
}) {
  const [text, setText] = useState("");
  return (
    <Card size="small" title={props.title}>
      <Space direction="vertical" style={{ width: "100%" }}>
        <Input.Search
          allowClear
          placeholder={props.placeholder}
          onChange={(e) => setText(e.target.value)}
          style={{ maxWidth: 360 }}
        />
        <Table
          size="small"
          rowKey="id"
          loading={props.loading}
          pagination={{ pageSize: 50, hideOnSinglePage: true }}
          columns={props.columns}
          dataSource={search(props.rows, text)}
        />
      </Space>
    </Card>
  );
}

export function SwitchNeighborsTab({ switchId }: { switchId: string }) {
  const { data = [], isLoading } = useSwitchNeighbors(switchId);
  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      <NeighborTable
        title="Tetangga LLDP"
        placeholder="Cari tetangga LLDP"
        rows={data.filter((n) => n.kind === "lldp")}
        columns={lldpColumns}
        loading={isLoading}
      />
      <NeighborTable
        title="Tabel MAC"
        placeholder="Cari MAC, VLAN atau port"
        rows={data.filter((n) => n.kind === "mac")}
        columns={macColumns}
        loading={isLoading}
      />
    </Space>
  );
}
