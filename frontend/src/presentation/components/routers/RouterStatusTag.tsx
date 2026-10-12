import { Space, Tag } from "antd";
import type { MikrotikRouter } from "@/domain/entities";

function routerTag(router: MikrotikRouter) {
  if (!router.enabled) return <Tag>Nonaktif</Tag>;
  if (router.status === "down")
    return <Tag color="red">SNMP tidak menjawab</Tag>;
  if (router.apiStatus === "certificate_changed")
    return <Tag color="orange">Sertifikat berubah</Tag>;
  if (router.apiStatus === "error") return <Tag color="orange">API error</Tag>;
  if (router.status === "up") return <Tag color="green">Up</Tag>;
  return <Tag>Belum terbaca</Tag>;
}

// A router can be up while a POP behind one of its VLANs is gone; that VLAN
// still reports up, so its silence is shown next to the router's status.
export function RouterStatusTag({ router }: { router: MikrotikRouter }) {
  return (
    <Space size={4} wrap>
      {routerTag(router)}
      {router.enabled && router.silentInterfaces > 0 && (
        <Tag color="red">{`${router.silentInterfaces} VLAN tanpa trafik`}</Tag>
      )}
    </Space>
  );
}
