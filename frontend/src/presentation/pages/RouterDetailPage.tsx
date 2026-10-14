import { Card, Result, Spin, Tabs } from "antd";
import { useParams } from "react-router-dom";
import { useMikrotikRouters } from "@/application/hooks/useMikrotikRouters";
import { BgpPathsTab } from "@/presentation/components/routers/BgpPathsTab";
import { FlowDestinationsTab } from "@/presentation/components/routers/FlowDestinationsTab";
import { InterfacesTab } from "@/presentation/components/routers/InterfacesTab";
import { RouterInfoTab } from "@/presentation/components/routers/RouterInfoTab";
import { RouterStatusTag } from "@/presentation/components/routers/RouterStatusTag";
import { UpstreamTab } from "@/presentation/components/routers/UpstreamTab";

export function RouterDetailPage() {
  const { id = "" } = useParams();
  const { data: routers = [], isLoading } = useMikrotikRouters();
  const router = routers.find((r) => r.id === id);

  if (isLoading) return <Spin />;
  if (!router) return <Result status="404" title="Router tidak ditemukan" />;

  return (
    <Card title={router.name} extra={<RouterStatusTag router={router} />}>
      <Tabs
        items={[
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
            children: (
              <BgpPathsTab routerId={router.id} routerName={router.name} />
            ),
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
        ]}
      />
    </Card>
  );
}
