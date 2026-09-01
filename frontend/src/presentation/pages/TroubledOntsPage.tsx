import { useState } from "react";
import { Card, Tabs } from "antd";
import { PageHeader } from "@/presentation/components/common/PageHeader";
import { useOlts, usePonHealth, useTroubledOnts } from "@/application/hooks";
import { TroubledFilterBar } from "@/presentation/components/onts/TroubledFilterBar";
import { TroubledOntTab } from "@/presentation/components/onts/TroubledOntTab";
import { TroubledPonTab } from "@/presentation/components/onts/TroubledPonTab";
import type { PonHealth, TroubledResult } from "@/domain/entities";

interface TabItemsArgs {
  troubled: { data: TroubledResult | undefined; isLoading: boolean };
  hours: number;
  status?: string;
  onStatusChange: (status?: string) => void;
  ponFilter?: { slot: number; port: number };
  onClearPonFilter: () => void;
  oltId?: string;
  ponHealth: PonHealth | undefined;
  ponLoading: boolean;
  onSelectPon: (slot: number, port: number) => void;
}

// Data for the Tabs `items` prop, kept out of the component so the
// component itself stays a short read: state, then wiring, then layout.
function buildTabItems(args: TabItemsArgs) {
  return [
    {
      key: "pelanggan",
      label: "Per Pelanggan",
      children: (
        <TroubledOntTab
          rows={args.troubled.data?.data ?? []}
          summary={args.troubled.data?.summary}
          isLoading={args.troubled.isLoading}
          hours={args.hours}
          status={args.status}
          onStatusChange={args.onStatusChange}
          ponFilter={args.ponFilter}
          onClearPonFilter={args.onClearPonFilter}
        />
      ),
    },
    {
      key: "pon",
      label: "Per PON",
      children: (
        <TroubledPonTab
          oltId={args.oltId}
          ponHealth={args.ponHealth}
          isLoading={args.ponLoading}
          onSelectPon={args.onSelectPon}
        />
      ),
    },
  ];
}

/**
 * TroubledOntsPage ranks subscribers by how much they have been churning.
 *
 * The ONT list answers "is this subscriber up", and the worst faults pass that
 * test every time it is asked: an ONU that drops and returns every few seconds
 * reads online whenever anyone looks. Counting the traps it sent is what makes
 * such a subscriber visible; the outage beside it says what the churn cost the
 * person paying for the line.
 *
 * Two tabs share the OLT and range picked above them: one ranks subscribers,
 * the other draws the fault tree that explains where the worst of them sit.
 * Picking a PON on the second carries its subscribers into the first, closing
 * the loop between "where is the fault" and "who is on it".
 */
export function TroubledOntsPage() {
  const [hours, setHours] = useState(24);
  const [oltId, setOltId] = useState<string | undefined>();
  const [status, setStatus] = useState<string | undefined>();
  const [tab, setTab] = useState("pelanggan");
  const [ponFilter, setPonFilter] = useState<{ slot: number; port: number }>();
  const troubled = useTroubledOnts(hours, oltId, status);
  const { data: olts } = useOlts();
  const ponHealthQuery = usePonHealth(oltId, hours, tab === "pon");

  const handleSelectPon = (slot: number, port: number) => {
    setPonFilter({ slot, port });
    setTab("pelanggan");
  };
  const items = buildTabItems({
    troubled,
    hours,
    status,
    onStatusChange: setStatus,
    ponFilter,
    onClearPonFilter: () => setPonFilter(undefined),
    oltId,
    ponHealth: ponHealthQuery.data,
    ponLoading: ponHealthQuery.isLoading,
    onSelectPon: handleSelectPon,
  });

  return (
    <div>
      <PageHeader
        title="Pelanggan Bermasalah"
        description="Diperingkat dari trap yang dikirim OLT, termasuk pelanggan yang statusnya terbaca online"
      />

      <Card
        extra={
          <TroubledFilterBar
            hours={hours}
            onHoursChange={setHours}
            oltId={oltId}
            onOltChange={setOltId}
            olts={olts ?? []}
          />
        }
      >
        <Tabs activeKey={tab} onChange={setTab} items={items} />
      </Card>
    </div>
  );
}

export default TroubledOntsPage;
