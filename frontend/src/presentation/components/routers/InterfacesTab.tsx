import { useState } from "react";
import { Card, Input, Space, Table, Tag } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { RouterInterface } from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { formatSpeed } from "../trafficFormat";
import { InterfaceTrafficModal } from "./InterfaceTrafficModal";
import {
  filterInterfaces,
  formatBps,
  groupInterfacesByParent,
  interfaceLabel,
  interfaceStatus,
  isUpstream,
} from "./routerTraffic";

const statusTag = (iface: RouterInterface) => {
  const { text, color } = interfaceStatus(iface);
  return <Tag color={color}>{text}</Tag>;
};

const columns: ColumnsType<RouterInterface> = [
  { title: "Label", render: (_, i) => interfaceLabel(i) },
  { title: "Nama", dataIndex: "name" },
  { title: "VLAN", render: (_, i) => i.vlanId ?? "—" },
  { title: "Status", render: (_, i) => statusTag(i) },
  {
    title: "Kecepatan",
    render: (_, i) => (i.speedMbps ? formatSpeed(i.speedMbps) : "—"),
  },
  { title: "Masuk (rx)", render: (_, i) => formatBps(i.lastInBps) },
  { title: "Keluar (tx)", render: (_, i) => formatBps(i.lastOutBps) },
];

export function InterfacesTab({ routerId }: { routerId: string }) {
  const { data: interfaces = [], isLoading } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const [search, setSearch] = useState("");
  const [selected, setSelected] = useState<RouterInterface | null>(null);
  const groups = groupInterfacesByParent(filterInterfaces(interfaces, search));

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      <Input.Search
        allowClear
        placeholder="Cari nama atau comment (mis. Cariu)"
        onChange={(e) => setSearch(e.target.value)}
        style={{ maxWidth: 360 }}
      />
      {groups.map((group) => (
        <Card key={group.parent} size="small" title={group.parent}>
          <Table
            size="small"
            rowKey="id"
            loading={isLoading}
            pagination={false}
            columns={columns}
            dataSource={group.interfaces}
            onRow={(iface) => ({
              onClick: () => setSelected(iface),
              style: { cursor: "pointer" },
            })}
          />
        </Card>
      ))}
      {selected && (
        <InterfaceTrafficModal
          routerId={routerId}
          iface={selected}
          upstream={isUpstream(selected, sessions)}
          onClose={() => setSelected(null)}
        />
      )}
    </Space>
  );
}
