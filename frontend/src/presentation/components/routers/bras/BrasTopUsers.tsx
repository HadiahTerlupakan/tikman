import { Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { BrasUser } from "@/domain/entities";
import { formatBps } from "../routerTraffic";

// RouterOS names a PPPoE session's dynamic queue "<pppoe-USER>"; the username
// inside is what an operator searches for. Other queues keep their name.
const PPPOE_QUEUE = /^<pppoe-(.+)>$/;

function userOf(queueName: string): string {
  return PPPOE_QUEUE.exec(queueName)?.[1] ?? queueName;
}

const columns: ColumnsType<BrasUser> = [
  {
    title: "Pelanggan",
    render: (_, u) => (
      <Typography.Text copyable>{userOf(u.name)}</Typography.Text>
    ),
  },
  {
    title: "Unduh",
    align: "right",
    render: (_, u) => formatBps(u.downBps),
  },
  {
    title: "Unggah",
    align: "right",
    render: (_, u) => formatBps(u.upBps),
  },
];

interface Props {
  users: BrasUser[];
  queuesFound: boolean;
}

/** The heaviest users by download, read from the dynamic queues RADIUS or a profile creates. */
export function BrasTopUsers({ users, queuesFound }: Props) {
  return (
    <section>
      <Typography.Title level={5}>Pemakai terbesar saat ini</Typography.Title>
      {queuesFound ? (
        <Table
          size="small"
          rowKey="name"
          pagination={false}
          columns={columns}
          dataSource={users}
        />
      ) : (
        <Typography.Text type="secondary">
          Tidak ada antrean dinamis: batas kecepatan tidak diberikan oleh RADIUS
          atau profil.
        </Typography.Text>
      )}
    </section>
  );
}
