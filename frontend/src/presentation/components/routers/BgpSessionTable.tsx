import { Alert, Card, Space, Table, Tag } from "antd";
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

interface Props {
  groups: SessionGroup[];
  stale: boolean;
  lastReadAt: string | null;
}

export function BgpSessionTable({ groups, stale, lastReadAt }: Props) {
  const readAt = lastReadAt ? dayjs(lastReadAt).format("DD MMM HH:mm:ss") : "—";
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {stale && (
        <Alert
          type="warning"
          showIcon
          message={`Data BGP terakhir terbaca ${readAt} — REST sedang gagal`}
        />
      )}
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
