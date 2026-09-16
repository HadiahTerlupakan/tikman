import { Card, Empty } from "antd";
import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { DayPerformance } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

interface DailyPerformanceChartProps {
  days: DayPerformance[];
  targetMinutes: number;
}

/** The share of answers within target, day by day. A day nobody was answered
 * leaves a gap rather than a drop to zero, which would read as a bad day. */
export function DailyPerformanceChart({
  days,
  targetMinutes,
}: DailyPerformanceChartProps) {
  const data = days.map((day) => ({
    date: day.date.slice(5),
    pct: day.replies.withinTargetPct,
  }));
  const measured = days.some((day) => day.replies.count > 0);

  return (
    <Card title={`Dibalas ≤ ${targetMinutes} menit per hari`}>
      {measured ? (
        <ResponsiveContainer width="100%" height={220}>
          <LineChart data={data}>
            <CartesianGrid strokeDasharray="3 3" opacity={0.2} />
            <XAxis dataKey="date" tick={{ fontSize: 10 }} />
            <YAxis domain={[0, 100]} unit="%" tick={{ fontSize: 10 }} />
            <Tooltip
              formatter={(value: unknown) =>
                typeof value === "number" ? `${Math.round(value)}%` : "—"
              }
            />
            <Line
              type="monotone"
              dataKey="pct"
              stroke={colors.success}
              connectNulls={false}
            />
          </LineChart>
        </ResponsiveContainer>
      ) : (
        <Empty description="Belum ada giliran yang dibalas di periode ini" />
      )}
    </Card>
  );
}
