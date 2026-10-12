import { DatePicker, Radio, Space } from "antd";
import dayjs from "dayjs";
import type { TrafficRange } from "@/domain/entities";
import { resolveTrafficRange } from "@/application/hooks/trafficRange";

const { RangePicker } = DatePicker;

// Presets live outside the RangePicker because its own presets arrive through
// onChange exactly like a manual pick, and only a preset should follow the clock.
const presets = [
  { label: "6 jam", value: 6 },
  { label: "24 jam", value: 24 },
  { label: "7 hari", value: 7 * 24 },
  { label: "30 hari", value: 30 * 24 },
];

interface Props {
  value: TrafficRange;
  onChange: (range: TrafficRange) => void;
}

export function TrafficRangePicker({ value, onChange }: Props) {
  const shown = resolveTrafficRange(value);
  return (
    <Space wrap>
      <Radio.Group
        optionType="button"
        buttonStyle="solid"
        options={presets}
        value={value.kind === "relative" ? value.hours : null}
        onChange={(e) =>
          onChange({ kind: "relative", hours: Number(e.target.value) })
        }
      />
      <RangePicker
        showTime
        value={[dayjs(shown.from), dayjs(shown.to)]}
        onChange={(dates) => {
          if (dates?.[0] && dates[1]) {
            onChange({
              kind: "absolute",
              from: dates[0].toISOString(),
              to: dates[1].toISOString(),
            });
          }
        }}
      />
    </Space>
  );
}
