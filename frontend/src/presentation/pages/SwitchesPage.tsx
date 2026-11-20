import { useState } from "react";
import { Button, Card, Spin, Typography } from "antd";
import { UserRole, type ManagedSwitch } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useSwitches } from "@/application/hooks/useSwitches";
import { SwitchFormModal } from "@/presentation/components/switches/SwitchFormModal";
import { SwitchRack } from "@/presentation/components/switches/SwitchRack";
import { switchReachable } from "@/presentation/components/switches/switchPorts";

function rackSummary(switches: ManagedSwitch[]): string {
  const reachable = switches.filter(switchReachable).length;
  const away = switches.length - reachable;
  const unreachable = away ? `, ${away} tidak terjangkau` : "";
  return `${switches.length} switch · ${reachable} terjangkau${unreachable}`;
}

export default function SwitchesPage() {
  const { data: switches = [], isLoading } = useSwitches();
  const isAdmin = useAuthStore((state) => state.user?.role) === UserRole.ADMIN;
  const [adding, setAdding] = useState(false);
  const [editing, setEditing] = useState<ManagedSwitch | null>(null);

  return (
    <Card
      title="Switch"
      extra={
        isAdmin && (
          <Button type="primary" onClick={() => setAdding(true)}>
            Tambah switch
          </Button>
        )
      }
    >
      <Spin spinning={isLoading}>
        <Typography.Paragraph type="secondary">
          {rackSummary(switches)}
        </Typography.Paragraph>
        <SwitchRack switches={switches} isAdmin={isAdmin} onEdit={setEditing} />
      </Spin>
      {adding && <SwitchFormModal onClose={() => setAdding(false)} />}
      {editing && (
        <SwitchFormModal sw={editing} onClose={() => setEditing(null)} />
      )}
    </Card>
  );
}
