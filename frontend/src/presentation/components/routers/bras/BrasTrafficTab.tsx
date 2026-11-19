import { useState } from "react";
import { Space, Typography } from "antd";
import type { MikrotikRouter } from "@/domain/entities";
import {
  useInterfaceTraffic,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { DEFAULT_TRAFFIC_RANGE } from "@/application/hooks/trafficRange";
import { TrafficChart } from "../TrafficChart";
import { TrafficRangePicker } from "../TrafficRangePicker";
import {
  directionLabels,
  interfaceLabel,
  interfaceSeries,
} from "../routerTraffic";

function UplinkTraffic({
  routerId,
  uplinkId,
}: {
  routerId: string;
  uplinkId: string;
}) {
  const [range, setRange] = useState(DEFAULT_TRAFFIC_RANGE);
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: points = [] } = useInterfaceTraffic(routerId, uplinkId, range);
  const uplink = interfaces.find((i) => i.id === uplinkId);
  // The uplink faces the border: what it receives is what subscribers download.
  const labels = directionLabels(true);
  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <TrafficRangePicker value={range} onChange={setRange} />
      <TrafficChart
        title={`Jalur ke border${uplink ? ` · ${interfaceLabel(uplink)}` : ""}`}
        series={[labels.in, labels.out]}
        data={interfaceSeries(points, labels)}
      />
    </Space>
  );
}

/** Traffic over the BRAS's one link to the border; a BRAS has no BGP upstreams of its own. */
export function BrasTrafficTab({ router }: { router: MikrotikRouter }) {
  if (!router.uplinkInterfaceId) {
    return (
      <Typography.Text type="secondary">
        Jalur ke border belum ditemukan. TikMan mencarinya dari default route
        saat discovery.
      </Typography.Text>
    );
  }
  return (
    <UplinkTraffic routerId={router.id} uplinkId={router.uplinkInterfaceId} />
  );
}
