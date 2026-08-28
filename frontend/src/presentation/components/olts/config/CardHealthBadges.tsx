import { Space, Tag, Tooltip } from "antd";
import type { CardHealth } from "@/domain/entities";

// Thresholds an operator would act on. A GPON line card idles in the fifties,
// so warning starts above that rather than at a generic 70.
const TEMP_WARNING_C = 60;
const TEMP_CRITICAL_C = 70;
const LOAD_WARNING_PERCENT = 75;
const LOAD_CRITICAL_PERCENT = 90;

function level(value: number, warning: number, critical: number): string {
  if (value >= critical) return "error";
  if (value >= warning) return "warning";
  return "success";
}

interface CardHealthBadgesProps {
  health?: CardHealth;
}

// Temperature, CPU and memory for one slot. A missing reading is left out
// rather than shown as zero: an empty slot reports 0% exactly like an idle
// card would, and the OLT sends -1000 for a temperature it cannot read.
export function CardHealthBadges({ health }: CardHealthBadgesProps) {
  if (!health) return null;

  const { temperatureC, cpuPercent, memoryPercent } = health;

  return (
    <Space size={4} wrap>
      {temperatureC !== undefined && (
        <Tooltip title="Card temperature">
          <Tag
            color={level(temperatureC, TEMP_WARNING_C, TEMP_CRITICAL_C)}
            data-testid={`card-temp-${health.slot}`}
          >
            {temperatureC}°C
          </Tag>
        </Tooltip>
      )}
      {cpuPercent !== undefined && (
        <Tooltip title="CPU load">
          <Tag
            color={level(
              cpuPercent,
              LOAD_WARNING_PERCENT,
              LOAD_CRITICAL_PERCENT,
            )}
          >
            CPU {cpuPercent}%
          </Tag>
        </Tooltip>
      )}
      {memoryPercent !== undefined && (
        <Tooltip title="Memory used">
          <Tag
            color={level(
              memoryPercent,
              LOAD_WARNING_PERCENT,
              LOAD_CRITICAL_PERCENT,
            )}
          >
            MEM {memoryPercent}%
          </Tag>
        </Tooltip>
      )}
    </Space>
  );
}
