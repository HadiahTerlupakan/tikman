import { Tooltip, theme } from "antd";
import { durasi } from "./troubledDuration";

/**
 * OutageBar shows the share of the window a subscriber had no service.
 *
 * A duration alone does not compare: "826 minutes" means one thing against a
 * day and another against a week. The proportion is what an operator ranking
 * subscribers is actually reading, so it is what the row draws.
 */
export function OutageBar({
  minutes,
  windowMinutes,
}: {
  minutes: number;
  windowMinutes: number;
}) {
  const { token } = theme.useToken();
  const share = Math.min(1, minutes / windowMinutes);

  return (
    <Tooltip title={`${Math.round(share * 100)}% dari rentang`}>
      <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
        <span style={{ fontVariantNumeric: "tabular-nums" }}>
          {durasi(minutes)}
        </span>
        <div
          aria-hidden
          style={{
            height: 3,
            borderRadius: 2,
            background: token.colorBorderSecondary,
            overflow: "hidden",
          }}
        >
          <div
            style={{
              width: `${share * 100}%`,
              height: "100%",
              background: share > 0.25 ? token.colorError : token.colorWarning,
            }}
          />
        </div>
      </div>
    </Tooltip>
  );
}
