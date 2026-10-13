import { Card, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import type { BgpState, RouterBgpEvent } from "@/domain/entities";

const stateTag = (state: BgpState) =>
  state === "established" ? (
    <Tag color="green">Established</Tag>
  ) : (
    <Tag color="red">Down</Tag>
  );

const columns: ColumnsType<RouterBgpEvent> = [
  {
    title: "Waktu",
    render: (_, e) => dayjs(e.occurredAt).format("DD MMM YYYY HH:mm:ss"),
  },
  { title: "Sesi", dataIndex: "sessionName" },
  {
    title: "Perubahan",
    render: (_, e) => (
      <>
        {stateTag(e.fromState)} → {stateTag(e.toState)}
      </>
    ),
  },
];

export function BgpEventList({ events }: { events: RouterBgpEvent[] }) {
  return (
    <Card size="small" title="Riwayat sesi BGP">
      <Table
        size="small"
        rowKey="id"
        columns={columns}
        dataSource={events}
        pagination={{ defaultPageSize: 20 }}
      />
    </Card>
  );
}
