import { useMemo, useState } from "react";
import { Space } from "antd";
import type { RouterApiStatus } from "@/domain/entities";
import {
  useInterfacesTraffic,
  useRouterBgpEvents,
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import { DEFAULT_TRAFFIC_RANGE } from "@/application/hooks/trafficRange";
import { BgpEventList } from "./BgpEventList";
import { BgpSessionTable } from "./BgpSessionTable";
import { TrafficChart } from "./TrafficChart";
import { TrafficRangePicker } from "./TrafficRangePicker";
import {
  groupSessionsByInterface,
  interfaceLabel,
  isUpstream,
  lastBgpRead,
  stackSeries,
} from "./routerTraffic";

interface Props {
  routerId: string;
  apiStatus: RouterApiStatus;
}

export function UpstreamTab({ routerId, apiStatus }: Props) {
  const [range, setRange] = useState(DEFAULT_TRAFFIC_RANGE);
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
        stale={apiStatus !== "ok"}
        lastReadAt={lastBgpRead(sessions)}
      />
      <BgpEventList events={events} />
    </Space>
  );
}
