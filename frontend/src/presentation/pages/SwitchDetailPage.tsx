import { Card, Result, Space, Spin, Tabs, Tag, Typography } from "antd";
import { useParams } from "react-router-dom";
import { UserRole } from "@/domain/entities";
import { useAuthStore } from "@/application/stores";
import { useSwitches } from "@/application/hooks/useSwitches";
import { SwitchConfigTab } from "@/presentation/components/switches/SwitchConfigTab";
import { SwitchFaceplate } from "@/presentation/components/switches/SwitchFaceplate";
import { SwitchHostKeyBanner } from "@/presentation/components/switches/SwitchHostKeyBanner";
import { SwitchNeighborsTab } from "@/presentation/components/switches/SwitchNeighborsTab";
import { SwitchPortsTab } from "@/presentation/components/switches/SwitchPortsTab";
import { SwitchVlanTab } from "@/presentation/components/switches/SwitchVlanTab";
import { switchReachable } from "@/presentation/components/switches/switchPorts";

const BACKUP_ROLES = [UserRole.ADMIN, UserRole.TECHNICIAN];

export default function SwitchDetailPage() {
  const { id = "" } = useParams();
  const { data: switches = [], isLoading } = useSwitches();
  const role = useAuthStore((state) => state.user?.role);
  const sw = switches.find((s) => s.id === id);

  if (isLoading) return <Spin />;
  if (!sw) return <Result status="404" title="Switch tidak ditemukan" />;

  const tabs = [
    {
      key: "panel",
      label: "Panel depan",
      children: <SwitchFaceplate sw={sw} />,
    },
    {
      key: "port",
      label: "Port",
      children: <SwitchPortsTab switchId={sw.id} />,
    },
    {
      key: "vlan",
      label: "VLAN",
      children: <SwitchVlanTab switchId={sw.id} />,
    },
    {
      key: "tetangga",
      label: "Tetangga",
      children: <SwitchNeighborsTab switchId={sw.id} />,
    },
    {
      key: "config",
      label: "Config",
      children: (
        <SwitchConfigTab
          switchId={sw.id}
          canBackup={role !== undefined && BACKUP_ROLES.includes(role)}
        />
      ),
    },
  ];

  return (
    <Card
      title={
        <Space>
          {sw.name}
          <Typography.Text type="secondary">{sw.host}</Typography.Text>
        </Space>
      }
      extra={
        switchReachable(sw) ? (
          <Tag color="green">Up</Tag>
        ) : (
          <Tag color="red">Tidak terjangkau</Tag>
        )
      }
    >
      <SwitchHostKeyBanner sw={sw} isAdmin={role === UserRole.ADMIN} />
      <Tabs items={tabs} />
    </Card>
  );
}
