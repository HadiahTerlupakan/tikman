import { Card, Space, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { RouterBgpSession } from "@/domain/entities";
import {
  formatUptime,
  interfaceLabel,
  type SessionGroup,
} from "./routerTraffic";

const columns: ColumnsType<RouterBgpSession> = [
  { title: "Sesi", dataIndex: "name" },
  {
    title: "Peer",
    render: (_, s) => `${s.remoteAddress} (AS${s.remoteAs})`,
  },
  {
    title: "State",
    render: (_, s) =>
      s.state === "established" ? (
        <Tag color="green">Established</Tag>
      ) : (
        <Tag color="red">Down</Tag>
      ),
  },
  { title: "Uptime", render: (_, s) => formatUptime(s.uptimeSeconds) },
  { title: "Prefix", render: (_, s) => s.prefixCount.toLocaleString("id-ID") },
  {
    title: "Terakhir putus",
    render: (_, s) =>
      s.lastStoppedAt ? dayjs(s.lastStoppedAt).format("DD MMM HH:mm:ss") : "—",
  },
];

export function BgpSessionTable({ groups }: { groups: SessionGroup[] }) {
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {groups.map((group) => (
        <Card
          key={group.iface?.id ?? "unmatched"}
          size="small"
          title={
            group.iface
              ? interfaceLabel(group.iface)
              : "Interface tidak diketahui"
          }
        >
          <Table
            size="small"
            rowKey="id"
            pagination={false}
            columns={columns}
            dataSource={group.sessions}
          />
        </Card>
      ))}
    </Space>
  );
}
