import { useState } from "react";
import { DatePicker, Segmented, Space, Spin, Typography } from "antd";
import { useCsPerformanceSummary } from "@/application/hooks/useCsPerformance";
import type { PerformancePeriod } from "@/domain/entities";
import { PageHeader } from "../components/common/PageHeader";
import { AgentPerformanceTable } from "../components/cs/performance/AgentPerformanceTable";
import { DailyPerformanceChart } from "../components/cs/performance/DailyPerformanceChart";
import { PerformanceTiles } from "../components/cs/performance/PerformanceTiles";
import { WaitListDrawer } from "../components/cs/performance/WaitListDrawer";
import {
  presetPeriod,
  type PeriodPreset,
} from "../components/cs/performance/performancePeriod";
import { formatPeriod } from "../components/cs/performance/performanceFormat";

const { RangePicker } = DatePicker;
const { Text } = Typography;

// Matches AgentPerformanceTable's own fallback for the same missing-user
// case, so a deleted CS is not named one way in the table and another in the
// drawer title it opens.
const DELETED_USER_LABEL = "Pengguna terhapus";

type PeriodChoice = PeriodPreset | "pilih-tanggal";

const PERIOD_OPTIONS: { label: string; value: PeriodChoice }[] = [
  { label: "Hari ini", value: "hari-ini" },
  { label: "7 hari", value: "7-hari" },
  { label: "Bulan ini", value: "bulan-ini" },
  { label: "Bulan lalu", value: "bulan-lalu" },
  { label: "Pilih tanggal", value: "pilih-tanggal" },
];

/** Which waits the list drawer shows: one CS's, or the whole team's. */
interface WaitListTarget {
  title: string;
  userId?: string;
}

/** How fast the team answers customers: the team's figures, each CS's, each
 * day's, and the waits behind them. */
export function CsPerformancePage() {
  const [choice, setChoice] = useState<PeriodChoice>("hari-ini");
  const [period, setPeriod] = useState<PerformancePeriod>(() =>
    presetPeriod("hari-ini", new Date()),
  );
  const [waitList, setWaitList] = useState<WaitListTarget | null>(null);
  const { data: summary, isLoading } = useCsPerformanceSummary(period);

  const choose = (value: PeriodChoice) => {
    setChoice(value);
    if (value !== "pilih-tanggal") {
      setPeriod(presetPeriod(value, new Date()));
    }
  };

  return (
    <Space direction="vertical" size="large" style={{ width: "100%" }}>
      <PageHeader
        title="Kinerja CS"
        description="Seberapa cepat pelanggan dibalas"
      />
      <Space wrap>
        <Segmented
          options={PERIOD_OPTIONS}
          value={choice}
          onChange={(value) => choose(value as PeriodChoice)}
        />
        {choice === "pilih-tanggal" && (
          <RangePicker
            allowEmpty={[false, false]}
            onChange={(values) => {
              if (values?.[0] && values[1]) {
                setPeriod({
                  from: values[0].format("YYYY-MM-DD"),
                  to: values[1].format("YYYY-MM-DD"),
                });
              }
            }}
          />
        )}
        {/* Always shown, not only while picking a custom range: without it,
            switching periods leaves the previous period's figures on screen
            with nothing saying which dates they are for. */}
        <Text type="secondary">{formatPeriod(period)}</Text>
      </Space>
      {!summary && isLoading && <Spin />}
      {summary && (
        <>
          <PerformanceTiles
            summary={summary}
            onShowWaits={() => setWaitList({ title: "Daftar giliran" })}
          />
          <AgentPerformanceTable
            agents={summary.agents}
            targetMinutes={summary.targetMinutes}
            loading={isLoading}
            onSelect={(agent) =>
              setWaitList({
                title: `Giliran ${agent.username || DELETED_USER_LABEL}`,
                userId: agent.userId,
              })
            }
          />
          <DailyPerformanceChart
            days={summary.days}
            targetMinutes={summary.targetMinutes}
          />
        </>
      )}
      {waitList && (
        <WaitListDrawer
          title={waitList.title}
          period={period}
          userId={waitList.userId}
          onClose={() => setWaitList(null)}
        />
      )}
    </Space>
  );
}
