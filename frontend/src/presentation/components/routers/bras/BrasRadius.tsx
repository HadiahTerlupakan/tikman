import { Space, Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { BrasRadius as RadiusServer } from "@/domain/entities";
import { BrasLed } from "./BrasSignals";
import { formatCount } from "./brasSignalModel";

function ServerStatus({ server }: { server: RadiusServer }) {
  if (server.answering === null) {
    return (
      <Space size={6}>
        <BrasLed level="none" />
        belum dinilai
      </Space>
    );
  }
  return (
    <Space size={6}>
      <BrasLed level={server.answering ? "ok" : "bad"} />
      {server.answering ? "menjawab" : "tidak menjawab"}
    </Space>
  );
}

const columns: ColumnsType<RadiusServer> = [
  { title: "Server", dataIndex: "address" },
  { title: "Status", render: (_, s) => <ServerStatus server={s} /> },
  {
    title: "Waktu jawab",
    align: "right",
    className: "bras-num",
    render: (_, s) => (s.rttMs === null ? "—" : `${s.rttMs} ms`),
  },
  {
    title: "Timeout",
    align: "right",
    className: "bras-num",
    render: (_, s) => formatCount(s.timeouts),
  },
];

/** RADIUS servers; none at all means PPP logs in from local secrets, which is fine. */
export function BrasRadius({ servers }: { servers: RadiusServer[] }) {
  return (
    <section>
      <Typography.Title level={5}>RADIUS</Typography.Title>
      {servers.length === 0 ? (
        <Typography.Text type="secondary">
          Tidak dipakai: login PPP tidak lewat RADIUS.
        </Typography.Text>
      ) : (
        <Table
          size="small"
          rowKey="id"
          pagination={false}
          columns={columns}
          dataSource={servers}
        />
      )}
    </section>
  );
}
