import { useMemo, useState } from "react";
import {
  Card,
  Table,
  Radio,
  Select,
  Empty,
  Skeleton,
  Space,
  Tabs,
  Tag,
  theme,
} from "antd";
import { PageHeader } from "@/presentation/components/common/PageHeader";
import { useOlts, usePonHealth, useTroubledOnts } from "@/application/hooks";
import { PonTopology } from "@/presentation/components/onts/PonTopology";
import {
  readsFineButIsNot,
  troubledColumns,
} from "@/presentation/components/onts/troubledColumns";
import { durasi } from "@/presentation/components/onts/troubledDuration";

const WINDOWS = [
  { label: "24 jam", hours: 24 },
  { label: "7 hari", hours: 168 },
];

/**
 * Summary states the case the table then makes row by row.
 *
 * The middle figure is the point of the whole page: subscribers the ONT list
 * reports as online, which lost service anyway inside the same window. Counted
 * over every matching ONT rather than the page shown — with hundreds churning
 * on one chassis, a total drawn from the fifty listed would mislead.
 */
function Summary({
  ontCount,
  hiddenByStatus,
  totalDownMinutes,
}: {
  ontCount: number;
  hiddenByStatus: number;
  totalDownMinutes: number;
}) {
  const { token } = theme.useToken();

  const figures = [
    { value: ontCount.toLocaleString("id-ID"), label: "pelanggan beralarm" },
    {
      value: hiddenByStatus.toLocaleString("id-ID"),
      label: "terbaca online, tetap sempat mati",
      accent: token.colorWarning,
    },
    { value: durasi(totalDownMinutes), label: "akumulasi waktu mati" },
  ];

  return (
    <div
      style={{
        display: "flex",
        flexWrap: "wrap",
        gap: 32,
        padding: "4px 0 20px",
      }}
    >
      {figures.map((figure) => (
        <div key={figure.label} style={{ minWidth: 150 }}>
          <div
            style={{
              fontSize: 26,
              fontWeight: 600,
              lineHeight: 1.2,
              fontVariantNumeric: "tabular-nums",
              color: figure.accent ?? token.colorText,
            }}
          >
            {figure.value}
          </div>
          <div style={{ fontSize: 12, color: token.colorTextSecondary }}>
            {figure.label}
          </div>
        </div>
      ))}
    </div>
  );
}

/**
 * TroubledOntsPage ranks subscribers by how much they have been churning.
 *
 * The ONT list answers "is this subscriber up", and the worst faults pass that
 * test every time it is asked: an ONU that drops and returns every few seconds
 * reads online whenever anyone looks. Counting the traps it sent is what makes
 * such a subscriber visible; the outage beside it says what the churn cost the
 * person paying for the line.
 */
export function TroubledOntsPage() {
  const { token } = theme.useToken();
  const [hours, setHours] = useState(24);
  const [oltId, setOltId] = useState<string | undefined>();
  const [status, setStatus] = useState<string | undefined>();
  const [tab, setTab] = useState("pelanggan");
  const [ponFilter, setPonFilter] = useState<{ slot: number; port: number }>();
  const { data, isLoading } = useTroubledOnts(hours, oltId, status);
  const { data: olts } = useOlts();
  const { data: ponHealth, isLoading: ponLoading } = usePonHealth(oltId, hours);

  const rows = data?.data ?? [];
  const summary = data?.summary;
  const worstTrapCount = rows[0]?.trapCount ?? 0;
  const hiddenByStatus = rows.filter(readsFineButIsNot).length;
  const shown = ponFilter
    ? rows.filter((r) => r.portId === ponFilter.port)
    : rows;

  // Chosen on the PON tab, applied on this one: the drill-down that closes
  // the loop between "where is the fault" and "who is on it".
  const handleSelectPon = (slot: number, port: number) => {
    setPonFilter({ slot, port });
    setTab("pelanggan");
  };

  const columns = useMemo(
    () =>
      troubledColumns(
        worstTrapCount,
        hours * 60,
        token.colorError,
        token.colorWarning,
      ),
    [worstTrapCount, hours, token.colorError, token.colorWarning],
  );

  return (
    <div>
      <PageHeader
        title="Pelanggan Bermasalah"
        description="Diperingkat dari trap yang dikirim OLT, termasuk pelanggan yang statusnya terbaca online"
      />

      <Card
        extra={
          <Space>
            <Select
              allowClear
              style={{ width: 170 }}
              placeholder="Semua OLT"
              value={oltId}
              onChange={setOltId}
              options={(olts ?? []).map((olt) => ({
                value: olt.id,
                label: olt.name,
              }))}
            />
            <Radio.Group
              value={hours}
              onChange={(e) => setHours(e.target.value)}
              optionType="button"
              buttonStyle="solid"
            >
              {WINDOWS.map((w) => (
                <Radio.Button key={w.hours} value={w.hours}>
                  {w.label}
                </Radio.Button>
              ))}
            </Radio.Group>
          </Space>
        }
      >
        <Tabs
          activeKey={tab}
          onChange={setTab}
          items={[
            {
              key: "pelanggan",
              label: "Per Pelanggan",
              children: (
                <>
                  <Space style={{ marginBottom: 16 }}>
                    <Select
                      allowClear
                      style={{ width: 150 }}
                      placeholder="Semua status"
                      value={status}
                      onChange={setStatus}
                      options={[
                        { value: "online", label: "Online" },
                        { value: "los", label: "LOS" },
                        { value: "dying_gasp", label: "Dying gasp" },
                        { value: "offline", label: "Offline" },
                      ]}
                    />
                    {ponFilter && (
                      <Tag
                        closable
                        onClose={() => setPonFilter(undefined)}
                      >{`PON ${ponFilter.port}`}</Tag>
                    )}
                  </Space>
                  {!isLoading && rows.length === 0 ? (
                    <Empty description="Tidak ada pelanggan yang beralarm dalam rentang ini" />
                  ) : (
                    <>
                      <Summary
                        ontCount={summary?.ontCount ?? 0}
                        hiddenByStatus={hiddenByStatus}
                        totalDownMinutes={summary?.totalDownMinutes ?? 0}
                      />
                      <Table
                        rowKey="ontId"
                        loading={isLoading}
                        dataSource={shown}
                        columns={columns}
                        size="small"
                        scroll={{ x: 900 }}
                        pagination={{ pageSize: 20, showSizeChanger: false }}
                        rowClassName={(record) =>
                          readsFineButIsNot(record)
                            ? "troubled-row-contradiction"
                            : ""
                        }
                      />
                    </>
                  )}
                </>
              ),
            },
            {
              key: "pon",
              label: "Per PON",
              children: !oltId ? (
                <Empty description="Pilih OLT untuk melihat topologinya" />
              ) : ponLoading || !ponHealth ? (
                <Skeleton active />
              ) : (
                <PonTopology health={ponHealth} onSelectPon={handleSelectPon} />
              ),
            },
          ]}
        />
      </Card>
    </div>
  );
}

export default TroubledOntsPage;
