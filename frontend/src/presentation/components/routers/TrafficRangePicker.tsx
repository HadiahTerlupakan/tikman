import { DatePicker } from "antd";
import dayjs, { type Dayjs } from "dayjs";
import type { TimeRange } from "@/domain/entities";

const { RangePicker } = DatePicker;

export const DEFAULT_RANGE_HOURS = 6;

export function lastHours(hours: number): TimeRange {
  const to = dayjs();
  return {
    from: to.subtract(hours, "hour").toISOString(),
    to: to.toISOString(),
  };
}

const presets: { label: string; value: () => [Dayjs, Dayjs] }[] = [
  { label: "6 jam", value: () => [dayjs().subtract(6, "hour"), dayjs()] },
  { label: "24 jam", value: () => [dayjs().subtract(1, "day"), dayjs()] },
  { label: "7 hari", value: () => [dayjs().subtract(7, "day"), dayjs()] },
  { label: "30 hari", value: () => [dayjs().subtract(30, "day"), dayjs()] },
];

interface Props {
  value: TimeRange;
  onChange: (range: TimeRange) => void;
}

export function TrafficRangePicker({ value, onChange }: Props) {
  return (
    <RangePicker
      showTime
      presets={presets}
      value={[dayjs(value.from), dayjs(value.to)]}
      onChange={(dates) => {
        if (dates?.[0] && dates[1]) {
          onChange({
            from: dates[0].toISOString(),
            to: dates[1].toISOString(),
          });
        }
      }}
    />
  );
}
