import { useMemo, useState } from "react";
import { Space } from "antd";
import {
  useInterfacesTraffic,
  useRouterBgpEvents,
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { BgpEventList } from "./BgpEventList";
import { BgpSessionTable } from "./BgpSessionTable";
import { TrafficChart } from "./TrafficChart";
import {
  DEFAULT_RANGE_HOURS,
  TrafficRangePicker,
  lastHours,
} from "./TrafficRangePicker";
import {
  groupSessionsByInterface,
  interfaceLabel,
  isUpstream,
  stackSeries,
} from "./routerTraffic";

export function UpstreamTab({ routerId }: { routerId: string }) {
  const [range, setRange] = useState(() => lastHours(DEFAULT_RANGE_HOURS));
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const { data: events = [] } = useRouterBgpEvents(routerId, range);

  const upstreams = useMemo(
    () =>
      interfaces
        .filter((i) => isUpstream(i, sessions))
        .map((iface) => ({ iface, label: interfaceLabel(iface) })),
    [interfaces, sessions],
  );
  const traffic = useInterfacesTraffic(routerId, upstreams, range);
  const labels = upstreams.map((u) => u.label);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <TrafficRangePicker value={range} onChange={setRange} />
      <TrafficChart
        stacked
        title="Download per upstream"
        series={labels}
        data={stackSeries(traffic, "inBps")}
      />
      <TrafficChart
        stacked
        title="Upload per upstream"
        series={labels}
        data={stackSeries(traffic, "outBps")}
      />
      <BgpSessionTable
        groups={groupSessionsByInterface(sessions, interfaces)}
      />
      <BgpEventList events={events} />
    </Space>
  );
}
