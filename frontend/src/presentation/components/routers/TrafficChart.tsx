import { Card, Empty } from "antd";
import dayjs from "dayjs";
import {
  Area,
  AreaChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
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

interface Props {
  title: string;
  data: ChartRow[];
  series: string[];
  stacked?: boolean;
}

export function TrafficChart({ title, data, series, stacked = false }: Props) {
  return (
    <Card size="small" title={title}>
      {data.length === 0 ? (
        <Empty description="Belum ada data di rentang ini" />
      ) : (
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
              labelFormatter={(t) =>
                dayjs(String(t)).format("DD MMM YYYY HH:mm")
              }
              formatter={(v) => formatBps(Number(v))}
            />
            <Legend />
            {series.map((name, i) => (
              <Area
                key={name}
                type="monotone"
                dataKey={name}
                stackId={stacked ? "stack" : undefined}
                stroke={COLORS[i % COLORS.length]}
                fill={COLORS[i % COLORS.length]}
                fillOpacity={stacked ? 0.6 : 0.2}
              />
            ))}
          </AreaChart>
        </ResponsiveContainer>
      )}
    </Card>
  );
}
