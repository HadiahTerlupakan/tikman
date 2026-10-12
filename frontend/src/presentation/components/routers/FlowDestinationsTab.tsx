import { useMemo, useState } from "react";
import { Alert, Space } from "antd";
import type { FlowFilter } from "@/domain/entities";
import {
  useRouterBgpSessions,
  useRouterInterfaces,
} from "@/application/hooks/useMikrotikRouters";
import {
  useFlowDestinations,
  useFlowPeering,
  useFlowPops,
  useFlowSummary,
  useFlowTopIps,
} from "@/application/hooks/useRouterFlows";
import { FlowFilters } from "./FlowFilters";
import { FlowPeeringDrawer } from "./FlowPeeringDrawer";
import { FlowPopTable } from "./FlowPopTable";
import { FlowSummaryTable } from "./FlowSummaryTable";
import { FlowTopIpsPanel } from "./FlowTopIpsPanel";
import { TrafficChart } from "./TrafficChart";
import { TrafficRangePicker } from "./TrafficRangePicker";
import {
  DEFAULT_FLOW_RANGE,
  FLOW_RANGE_PRESETS,
  flowChartRows,
  peeringLabels,
  topIpsAvailable,
} from "./routerFlows";
import { isUpstream } from "./routerTraffic";

const NOT_ENABLED =
  "NetFlow belum diaktifkan untuk router ini. Admin dapat mengaktifkannya lewat Ubah di tab Router, lalu nyalakan traffic-flow di router (lihat panduan operator).";

function LoadError({ what }: { what: string }) {
  return <Alert type="error" showIcon message={`Gagal memuat ${what}`} />;
}

interface Props {
  routerId: string;
  flowEnabled: boolean;
}

export function FlowDestinationsTab({ routerId, flowEnabled }: Props) {
  const [range, setRange] = useState(DEFAULT_FLOW_RANGE);
  const [filter, setFilter] = useState<FlowFilter>({ direction: "download" });
  const { data: interfaces = [] } = useRouterInterfaces(routerId);
  const { data: sessions = [] } = useRouterBgpSessions(routerId);
  const labels = useMemo(
    () => peeringLabels(interfaces, sessions),
    [interfaces, sessions],
  );
  const [selected, setSelected] = useState<{
    asn: number;
    name: string;
  } | null>(null);
  const upstreams = interfaces.filter((i) => isUpstream(i, sessions));
  const pops = interfaces.filter((i) => !isUpstream(i, sessions));

  const destinations = useFlowDestinations(routerId, range, filter);
  const summary = useFlowSummary(routerId, range, filter);
  const popRows = useFlowPops(routerId, range, filter.direction);
  const ipsAvailable = topIpsAvailable(range);
  const topIps = useFlowTopIps(routerId, range, filter, ipsAvailable);
  const peering = useFlowPeering(
    routerId,
    range,
    filter,
    selected?.asn ?? null,
  );
  const chart = flowChartRows(destinations.data);

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      {!flowEnabled && <Alert type="info" showIcon message={NOT_ENABLED} />}
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
        labels={labels}
      />
      {destinations.isError ? (
        <LoadError what="data tujuan trafik" />
      ) : (
        <TrafficChart
          stacked
          title="Tujuan trafik (bit/s)"
          series={chart.series}
          data={chart.rows}
        />
      )}
      {summary.isError ? (
        <LoadError what="ringkasan tujuan" />
      ) : (
        <FlowSummaryTable
          data={summary.data}
          labels={labels}
          loading={summary.isLoading}
          onSelect={(row) => setSelected({ asn: row.asn, name: row.name })}
        />
      )}
      {popRows.isError ? (
        <LoadError what="data per POP" />
      ) : (
        <FlowPopTable
          rows={popRows.data ?? []}
          labels={labels}
          loading={popRows.isLoading}
          onSelect={(pop) => setFilter({ ...filter, pop })}
        />
      )}
      <FlowTopIpsPanel
        available={ipsAvailable}
        rows={topIps.data ?? []}
        loading={topIps.isLoading}
        failed={topIps.isError}
      />
      {selected && (
        <FlowPeeringDrawer
          asn={selected.asn}
          name={selected.name}
          data={peering.data}
          loading={peering.isLoading}
          failed={peering.isError}
          labels={labels}
          onClose={() => setSelected(null)}
        />
      )}
    </Space>
  );
}
