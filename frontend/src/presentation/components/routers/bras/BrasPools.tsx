import { Progress, Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { BrasPool } from "@/domain/entities";
import { formatCount, poolLevel, type SignalLevel } from "./brasSignalModel";

const PERCENT = 100;
// The faceplate's LED colours, with the app's accent for a pool that is fine.
const BAR_COLOR: Record<SignalLevel, string> = {
  ok: "#2f6fe4",
  warn: "#f3b437",
  bad: "#ef4b4f",
  none: "#2f6fe4",
};

function PoolName({ pool }: { pool: BrasPool }) {
  if (pool.total === null || pool.free === null || pool.total === 0) {
    return <>{pool.name}</>;
  }
  return (
    <>
      {pool.name}
      <Progress
        size="small"
        showInfo={false}
        percent={(pool.used / pool.total) * PERCENT}
        strokeColor={BAR_COLOR[poolLevel(pool.free, pool.total)]}
      />
    </>
  );
}

const columns: ColumnsType<BrasPool> = [
  { title: "Pool", render: (_, p) => <PoolName pool={p} /> },
  {
    title: "Terpakai",
    align: "right",
    className: "bras-num",
    render: (_, p) => `${formatCount(p.used)} / ${formatCount(p.total)}`,
  },
  {
    title: "Sisa",
    align: "right",
    className: "bras-num",
    render: (_, p) => formatCount(p.free),
  },
];

export function BrasPools({ pools }: { pools: BrasPool[] }) {
  return (
    <section>
      <Typography.Title level={5}>Pool alamat</Typography.Title>
      <Table
        size="small"
        rowKey="name"
        pagination={false}
        columns={columns}
        dataSource={pools}
        locale={{ emptyText: "Tidak ada pool" }}
      />
    </section>
  );
}
