import { useState } from "react";
import { Modal, Space } from "antd";
import type { RouterInterface } from "@/domain/entities";
import { useInterfaceTraffic } from "@/application/hooks/useMikrotikRouters";
import { TrafficChart } from "./TrafficChart";
import { DEFAULT_TRAFFIC_RANGE } from "@/application/hooks/trafficRange";
import { TrafficRangePicker } from "./TrafficRangePicker";
import {
  directionLabels,
  interfaceLabel,
  interfaceSeries,
} from "./routerTraffic";

interface Props {
  routerId: string;
  iface: RouterInterface;
  upstream: boolean;
  onClose: () => void;
}

export function InterfaceTrafficModal({
  routerId,
  iface,
  upstream,
  onClose,
}: Props) {
  const [range, setRange] = useState(DEFAULT_TRAFFIC_RANGE);
  const { data: points = [] } = useInterfaceTraffic(routerId, iface.id, range);
  const labels = directionLabels(upstream);

  return (
    <Modal
      open
      width={960}
      footer={null}
      title={interfaceLabel(iface)}
      onCancel={onClose}
    >
      <Space direction="vertical" style={{ width: "100%" }}>
        <TrafficRangePicker value={range} onChange={setRange} />
        <TrafficChart
          title={`${iface.name}${iface.vlanId ? ` · VLAN ${iface.vlanId}` : ""}`}
          series={[labels.in, labels.out]}
          data={interfaceSeries(points, labels)}
        />
      </Space>
    </Modal>
  );
}
