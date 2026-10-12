import { Card, Empty, Segmented } from "antd";
import dayjs from "dayjs";
import { useState } from "react";
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import { ChartStatsTable } from "./ChartStatsTable";
import { formatBps, type ChartRow } from "./routerTraffic";

const COLORS = [
  "#1677ff",
  "#52c41a",
  "#fa8c16",
  "#eb2f96",
  "#722ed1",
  "#13c2c2",
  "#2f54eb",
  "#a0d911",
  "#faad14",
  "#f5222d",
  "#8c8c8c",
];
const CHART_HEIGHT = 280;
const LINE_FILL_OPACITY = 0.05;
const STACK_FILL_OPACITY = 0.6;

type Mode = "line" | "stack";

const MODE_OPTIONS = [
  { label: "Garis", value: "line" },
  { label: "Bertumpuk", value: "stack" },
];

interface Props {
  title: string;
  data: ChartRow[];
  series: string[];
}

interface AreasProps {
  data: ChartRow[];
  series: string[];
  colors: string[];
  hidden: Set<string>;
  stacked: boolean;
}

function TrafficAreas({ data, series, colors, hidden, stacked }: AreasProps) {
  return (
    <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
      <AreaChart data={data}>
        <CartesianGrid strokeDasharray="3 3" />
        <XAxis
          dataKey="time"
          minTickGap={40}
          tickFormatter={(t) => dayjs(t).format("DD/MM HH:mm")}
        />
        <YAxis width={90} tickFormatter={(v) => formatBps(Number(v))} />
        <Tooltip
          labelFormatter={(t) => dayjs(String(t)).format("DD MMM YYYY HH:mm")}
          formatter={(v) => formatBps(Number(v))}
        />
        {series.map((name, i) => (
          <Area
            key={name}
            type="monotone"
            dataKey={name}
            hide={hidden.has(name)}
            stackId={stacked ? "stack" : undefined}
            stroke={colors[i]}
            strokeWidth={stacked ? 1 : 1.5}
            fill={colors[i]}
            fillOpacity={stacked ? STACK_FILL_OPACITY : LINE_FILL_OPACITY}
          />
        ))}
      </AreaChart>
    </ResponsiveContainer>
  );
}

/**
 * Lines by default: each series at its own value, so a smaller link sits
 * below a larger one at its real height. Stacking reads as a total but puts
 * every line above the ones before it, which misled operators into reading a
 * small peering as the largest.
 */
export function TrafficChart({ title, data, series }: Props) {
  const [mode, setMode] = useState<Mode>("line");
  const [hidden, setHidden] = useState<Set<string>>(new Set());
  const colors = series.map((_, i) => COLORS[i % COLORS.length]);

  const toggle = (name: string) =>
    setHidden((prev) => {
      const next = new Set(prev);
      if (next.has(name)) next.delete(name);
      else next.add(name);
      return next;
    });

  const modeSwitch = (
    <Segmented
      size="small"
      options={MODE_OPTIONS}
      value={mode}
      onChange={(v) => setMode(v as Mode)}
    />
  );
  return (
    <Card size="small" title={title} extra={modeSwitch}>
      {data.length === 0 ? (
        <Empty description="Belum ada data di rentang ini" />
      ) : (
        <>
          <TrafficAreas
            data={data}
            series={series}
            colors={colors}
            hidden={hidden}
            stacked={mode === "stack"}
          />
          <ChartStatsTable
            data={data}
            series={series}
            colors={colors}
            hidden={hidden}
            onToggle={toggle}
          />
        </>
      )}
    </Card>
  );
}
