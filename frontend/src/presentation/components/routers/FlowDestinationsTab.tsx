import { useMemo, useState } from "react";
import { Space } from "antd";
import type { FlowFilter } from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import {
  useFlowDestinations,
  useFlowPops,
  useFlowSummary,
  useFlowTopIps,
} from "@/application/hooks/useRouterFlows";
import { FlowFilters } from "./FlowFilters";
import { FlowPopTable } from "./FlowPopTable";
import { FlowSummaryTable } from "./FlowSummaryTable";
import { FlowTopIpsPanel } from "./FlowTopIpsPanel";
import { TrafficChart } from "./TrafficChart";
import { TrafficRangePicker } from "./TrafficRangePicker";
import {
  DEFAULT_FLOW_RANGE,
  FLOW_RANGE_PRESETS,
  flowChartRows,
  topIpsAvailable,
} from "./routerFlows";
import { interfaceLabel, isUpstream } from "./routerTraffic";

export function FlowDestinationsTab({ routerId }: { routerId: string }) {
  const [range, setRange] = useState(DEFAULT_FLOW_RANGE);
  const [filter, setFilter] = useState<FlowFilter>({ direction: "download" });
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const labels = useMemo(
    () => new Map(interfaces.map((i) => [i.id, interfaceLabel(i)])),
    [interfaces],
  );
  const upstreams = interfaces.filter((i) => isUpstream(i, sessions));
  const pops = interfaces.filter((i) => !isUpstream(i, sessions));

  const destinations = useFlowDestinations(routerId, range, filter);
  const summary = useFlowSummary(routerId, range, filter);
  const popRows = useFlowPops(routerId, range, filter.direction);
  const ipsAvailable = topIpsAvailable(range);
  const topIps = useFlowTopIps(routerId, range, filter, ipsAvailable);
  const chart = flowChartRows(destinations.data);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <TrafficRangePicker
        value={range}
        onChange={setRange}
        presets={FLOW_RANGE_PRESETS}
      />
      <FlowFilters
        filter={filter}
        onChange={setFilter}
        pops={pops}
        upstreams={upstreams}
      />
      <TrafficChart
        stacked
        title="Tujuan trafik (bit/s)"
        series={chart.series}
        data={chart.rows}
      />
      <FlowSummaryTable
        data={summary.data}
        labels={labels}
        loading={summary.isLoading}
      />
      <FlowPopTable
        rows={popRows.data ?? []}
        labels={labels}
        loading={popRows.isLoading}
        onSelect={(pop) => setFilter({ ...filter, pop })}
      />
      <FlowTopIpsPanel
        available={ipsAvailable}
        rows={topIps.data ?? []}
        loading={topIps.isLoading}
      />
    </Space>
  );
}
