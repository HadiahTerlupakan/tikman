import { Radio, Select, Space } from "antd";
import type { Olt } from "@/domain/entities";

const WINDOWS = [
  { label: "24 jam", hours: 24 },
  { label: "7 hari", hours: 168 },
];

interface TroubledFilterBarProps {
  hours: number;
  onHoursChange: (hours: number) => void;
  oltId?: string;
  onOltChange: (oltId?: string) => void;
  olts: Olt[];
}

/**
 * TroubledFilterBar picks the OLT and the time window: the two axes both
 * tabs on the page share, which is why they sit above the Tabs rather than
 * inside either one.
 */
export function TroubledFilterBar({
  hours,
  onHoursChange,
  oltId,
  onOltChange,
  olts,
}: TroubledFilterBarProps) {
  return (
    <Space>
      <Select
        allowClear
        style={{ width: 170 }}
        placeholder="Semua OLT"
        value={oltId}
        onChange={onOltChange}
        options={olts.map((olt) => ({ value: olt.id, label: olt.name }))}
      />
      <Radio.Group
        value={hours}
        onChange={(e) => onHoursChange(e.target.value)}
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
  );
}
