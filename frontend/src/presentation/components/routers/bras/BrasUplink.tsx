import { Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { RouterInterface } from "@/domain/entities";
import { useRouterInterfaces } from "@/application/hooks/useMikrotikRouters";
import { formatSpeed } from "../../trafficFormat";
import { formatBps, interfaceLabel } from "../routerTraffic";

const columns: ColumnsType<RouterInterface> = [
  { title: "Port", render: (_, i) => interfaceLabel(i) },
  {
    title: "Unduh",
    align: "right",
    className: "bras-num",
    render: (_, i) => formatBps(i.lastInBps),
  },
  {
    title: "Unggah",
    align: "right",
    className: "bras-num",
    render: (_, i) => formatBps(i.lastOutBps),
  },
  {
    title: "Kapasitas",
    align: "right",
    className: "bras-num",
    render: (_, i) => (i.speedMbps ? formatSpeed(i.speedMbps) : "—"),
  },
];

interface Props {
  routerId: string;
  uplinkInterfaceId: string;
}

/** The BRAS's way to the border, found from its default route at discovery. */
export function BrasUplink({ routerId, uplinkInterfaceId }: Props) {
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const uplink = interfaces.find((i) => i.id === uplinkInterfaceId);
  if (!uplink) return null;
  return (
    <section>
      <Typography.Title level={5}>Jalur ke border</Typography.Title>
      <Table
        size="small"
        rowKey="id"
        pagination={false}
        columns={columns}
        dataSource={[uplink]}
      />
    </section>
  );
}
