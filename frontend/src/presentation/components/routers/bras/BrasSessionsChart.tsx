import { Empty } from "antd";
import dayjs from "dayjs";
import {
  CartesianGrid,
  Line,
  LineChart,
  ReferenceDot,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";
import type { BrasDrop, BrasSample } from "@/domain/entities";
import { dropPoints, sessionsSeries } from "./brasSeries";
import { formatCount } from "./brasSignalModel";

const CHART_HEIGHT = 230;
const LINE_COLOR = "#2f6fe4";
// The faceplate's down LED, so a drop reads as the same red as a dead port.
const DROP_COLOR = "#ef4b4f";
const DROP_TEXT = "#c4363a";
const DROP_RADIUS = 4.5;

interface Props {
  samples: BrasSample[];
  drops: BrasDrop[];
}

export function BrasSessionsChart({ samples, drops }: Props) {
  if (samples.length === 0) {
    return <Empty description="Belum ada data di rentang ini" />;
  }
  return (
    <ResponsiveContainer width="100%" height={CHART_HEIGHT}>
      <LineChart data={sessionsSeries(samples)}>
        <CartesianGrid strokeDasharray="3 3" vertical={false} />
        <XAxis
          dataKey="t"
          type="number"
          scale="time"
          domain={["dataMin", "dataMax"]}
          minTickGap={40}
          tickFormatter={(t) => dayjs(Number(t)).format("HH:mm")}
        />
        <YAxis width={56} tickFormatter={(v) => formatCount(Number(v))} />
        <Tooltip
          labelFormatter={(t) => dayjs(Number(t)).format("DD MMM HH:mm")}
          formatter={(v) => [formatCount(Number(v)), "Sesi"]}
        />
        <Line
          type="monotone"
          dataKey="sessions"
          stroke={LINE_COLOR}
          strokeWidth={1.6}
          dot={false}
          connectNulls={false}
          isAnimationActive={false}
        />
        {dropPoints(drops, samples).map((d) => (
          <ReferenceDot
            key={d.t}
            x={d.t}
            y={d.sessions}
            r={DROP_RADIUS}
            fill={DROP_COLOR}
            stroke="none"
            label={{
              value: d.label,
              position: "right",
              fill: DROP_TEXT,
              fontSize: 11,
            }}
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}
