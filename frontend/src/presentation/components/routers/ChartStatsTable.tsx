import { Table, Typography } from "antd";
import type { ColumnsType } from "antd/es/table";
import dayjs from "dayjs";
import { chartStats, type SeriesStats } from "./chartStats";
import { formatBps, type ChartRow } from "./routerTraffic";

const SWATCH_SIZE = 10;
const HIDDEN_OPACITY = 0.4;
const RANGE_FORMAT = "DD MMM YYYY HH:mm";

type StatsRow = SeriesStats & { color: string };

interface Props {
  data: ChartRow[];
  series: string[];
  colors: string[];
  hidden: Set<string>;
  onToggle: (name: string) => void;
}

interface ToggleProps {
  row: StatsRow;
  hidden: boolean;
  onToggle: (name: string) => void;
}

function SeriesToggle({ row, hidden, onToggle }: ToggleProps) {
  return (
    <Typography.Link
      onClick={() => onToggle(row.name)}
      style={{ color: "inherit", opacity: hidden ? HIDDEN_OPACITY : 1 }}
      title={hidden ? "Tampilkan garis" : "Sembunyikan garis"}
    >
      <span
        style={{
          display: "inline-block",
          width: SWATCH_SIZE,
          height: SWATCH_SIZE,
          marginRight: 8,
          background: row.color,
        }}
      />
      {row.name}
    </Typography.Link>
  );
}

function statsColumns(
  hidden: Set<string>,
  onToggle: (name: string) => void,
): ColumnsType<StatsRow> {
  const bps = (title: string, dataIndex: keyof SeriesStats) => ({
    title,
    dataIndex,
    align: "right" as const,
    render: formatBps,
  });
  return [
    {
      title: "Seri",
      dataIndex: "name",
      render: (_: string, row) => (
        <SeriesToggle
          row={row}
          hidden={hidden.has(row.name)}
          onToggle={onToggle}
        />
      ),
    },
    bps("Saat ini", "current"),
    bps("Rata-rata", "average"),
    bps("Maksimum", "maximum"),
  ];
}

/** Under the chart, Cacti-style: each line's current, mean and peak. */
export function ChartStatsTable({
  data,
  series,
  colors,
  hidden,
  onToggle,
}: Props) {
  const rows = chartStats(data, series).map((s, i) => ({
    ...s,
    color: colors[i],
  }));
  return (
    <>
      <Table
        size="small"
        rowKey="name"
        pagination={false}
        columns={statsColumns(hidden, onToggle)}
        dataSource={rows}
      />
      <Typography.Text type="secondary" style={{ fontSize: 12 }}>
        Dari {dayjs(String(data[0].time)).format(RANGE_FORMAT)} sampai{" "}
        {dayjs(String(data[data.length - 1].time)).format(RANGE_FORMAT)}
      </Typography.Text>
    </>
  );
}
