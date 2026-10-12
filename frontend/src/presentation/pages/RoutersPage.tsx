import { useState } from "react";
import { Button, Card, Table, Tooltip } from "antd";
import type { ColumnsType } from "antd/es/table";
import { Link } from "react-router-dom";
import { UserRole, type MikrotikRouter } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { RouterFormModal } from "@/presentation/components/routers/RouterFormModal";
import { RouterStatusTag } from "@/presentation/components/routers/RouterStatusTag";
import { bgpSummary } from "@/presentation/components/routers/routerTraffic";

const columns: ColumnsType<MikrotikRouter> = [
  {
    title: "Nama",
    render: (_, r) => <Link to={`/routers/${r.id}`}>{r.name}</Link>,
  },
  { title: "Host", dataIndex: "host" },
  { title: "Status", render: (_, r) => <RouterStatusTag router={r} /> },
  {
    title: "BGP",
    render: (_, r) =>
      r.apiStatus === "ok" ? (
        bgpSummary(r)
      ) : (
        <Tooltip title="API router sedang gagal; jumlah sesi terakhir sudah usang">
          <span>{bgpSummary(r)}</span>
        </Tooltip>
      ),
  },
  {
    title: "CPU",
    render: (_, r) => (r.cpuLoad === null ? "—" : `${r.cpuLoad}%`),
  },
  { title: "RouterOS", render: (_, r) => r.rosVersion ?? "—" },
];

export default function RoutersPage() {
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const isAdmin = useAuthStore((state) => state.user?.role) === UserRole.ADMIN;
  const [adding, setAdding] = useState(false);

  return (
    <Card
      title="MikroTik"
      extra={
        isAdmin && (
          <Button type="primary" onClick={() => setAdding(true)}>
            Tambah router
          </Button>
        )
      }
    >
      <Table
        rowKey="id"
        loading={isLoading}
        columns={columns}
        dataSource={routers}
        pagination={false}
      />
      {adding && <RouterFormModal onClose={() => setAdding(false)} />}
    </Card>
  );
}
