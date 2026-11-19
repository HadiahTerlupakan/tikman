import { useState } from "react";
import { Button, Card, Spin, Typography } from "antd";
import { UserRole, type MikrotikRouter } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { RouterFormModal } from "@/presentation/components/routers/RouterFormModal";
import { RouterRack } from "@/presentation/components/routers/faceplate/RouterRack";
import { isBras } from "@/presentation/components/routers/routerRole";
import { routerReachable } from "@/presentation/components/routers/faceplate/portModel";

function rackSummary(routers: MikrotikRouter[]): string {
  const reachable = routers.filter(routerReachable).length;
  const unreachable = routers.length - reachable;
  const bgp = routers.filter((r) => !isBras(r));
  const est = bgp.reduce((n, r) => n + r.bgpEstablished, 0);
  const total = bgp.reduce((n, r) => n + r.bgpTotal, 0);
  const brases = routers.filter(isBras);
  const online = brases.reduce((n, r) => n + (r.bras?.sessions ?? 0), 0);
  const customers = brases.length
    ? ` · ${online.toLocaleString("id-ID")} pelanggan online`
    : "";
  const away = unreachable ? `, ${unreachable} tidak terjangkau` : "";
  return `${routers.length} router · ${reachable} terjangkau${away} · BGP ${est}/${total} established${customers}`;
}

export default function RoutersPage() {
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const isAdmin = useAuthStore((state) => state.user?.role) === UserRole.ADMIN;
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<MikrotikRouter | null>(null);

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
      <Spin spinning={isLoading}>
        <Typography.Paragraph type="secondary">
          {rackSummary(routers)}
        </Typography.Paragraph>
        <RouterRack routers={routers} isAdmin={isAdmin} onEdit={setEditing} />
      </Spin>
      {adding && <RouterFormModal onClose={() => setAdding(false)} />}
      {editing && (
        <RouterFormModal router={editing} onClose={() => setEditing(null)} />
      )}
    </Card>
  );
}
