import { Card, Result, Space, Spin, Tabs } from "antd";
import { useParams } from "react-router-dom";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { BrasSummaryTab } from "@/presentation/components/routers/bras/BrasSummaryTab";
import { BrasTrafficTab } from "@/presentation/components/routers/bras/BrasTrafficTab";
import { BgpPathsTab } from "@/presentation/components/routers/BgpPathsTab";
import { FlowDestinationsTab } from "@/presentation/components/routers/FlowDestinationsTab";
import { RouterFaceplate } from "@/presentation/components/routers/faceplate/RouterFaceplate";
import { InterfacesTab } from "@/presentation/components/routers/InterfacesTab";
import { RouterInfoTab } from "@/presentation/components/routers/RouterInfoTab";
import { RoleTape } from "@/presentation/components/routers/RoleTape";
import { RouterStatusTag } from "@/presentation/components/routers/RouterStatusTag";
import { tabKeysFor } from "@/presentation/components/routers/routerRole";
import { UpstreamTab } from "@/presentation/components/routers/UpstreamTab";

export function RouterDetailPage() {
  const { id = "" } = useParams();
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const router = routers.find((r) => r.id === id);

  if (isLoading) return <Spin />;
  if (!router) return <Result status="404" title="Router tidak ditemukan" />;

  const keys = tabKeysFor(router.role);
  const tabs = [
    {
      key: "ringkasan",
      label: "Ringkasan",
      children: <BrasSummaryTab router={router} />,
    },
    {
      key: "panel",
      label: "Panel depan",
      children: <RouterFaceplate router={router} />,
    },
    {
      key: "trafik",
      label: "Trafik",
      children: <BrasTrafficTab router={router} />,
    },
    {
      key: "upstream",
      label: "Upstream & BGP",
      children: (
        <UpstreamTab routerId={router.id} apiStatus={router.apiStatus} />
      ),
    },
    {
      key: "paths",
      label: "Jalur BGP",
      children: <BgpPathsTab routerId={router.id} routerName={router.name} />,
    },
    {
      key: "flows",
      label: "Tujuan Trafik",
      children: (
        <FlowDestinationsTab
          routerId={router.id}
          flowEnabled={router.flowEnabled}
        />
      ),
    },
    {
      key: "interfaces",
      label: "Interface & VLAN",
      children: <InterfacesTab routerId={router.id} />,
    },
    {
      key: "router",
      label: "Router",
      children: <RouterInfoTab router={router} />,
    },
  ]
    .filter((tab) => keys.includes(tab.key))
    .sort((a, b) => keys.indexOf(a.key) - keys.indexOf(b.key));

  return (
    <Card
      title={
        <Space>
          {router.name}
          <RoleTape role={router.role} />
        </Space>
      }
      extra={<RouterStatusTag router={router} />}
    >
      <Tabs items={tabs} />
    </Card>
  );
}
