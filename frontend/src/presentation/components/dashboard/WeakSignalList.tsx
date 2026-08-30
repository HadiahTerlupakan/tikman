import { Skeleton } from "antd";
import { colors, statusSurfaces } from "@/shared/theme";
import type { WeakSignal } from "@/presentation/pages/dashboardStats";
import { signalTone } from "@/presentation/pages/dashboardStats";

interface WeakSignalListProps {
  signals: WeakSignal[];
  isLoading?: boolean;
}

/**
 * The online ONTs receiving the least light, weakest first. Optical margin
 * decays before a link drops, so this is the one panel on the page that shows a
 * fault while there is still time to act on it.
 */
export function WeakSignalList({ signals, isLoading }: WeakSignalListProps) {
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 4 }} title={false} />;
  }

  if (signals.length === 0) {
    return (
      <div style={{ color: colors.textSecondary, fontSize: 13 }}>
        No online ONT is reporting an optical reading.
      </div>
    );
  }

  return (
    <div style={{ display: "flex", flexDirection: "column" }}>
      {signals.map((signal, index) => (
        <div
          key={signal.id}
          style={{
            display: "flex",
            alignItems: "center",
            justifyContent: "space-between",
            gap: 12,
            padding: "10px 0",
            borderTop: index === 0 ? "none" : `1px solid ${colors.border}`,
          }}
        >
          <div style={{ minWidth: 0 }}>
            <div
              style={{
                color: colors.textBody,
                fontSize: 13,
                overflow: "hidden",
                textOverflow: "ellipsis",
                whiteSpace: "nowrap",
              }}
            >
              {signal.name}
            </div>
            <div style={{ color: colors.textMuted, fontSize: 11 }}>
              {signal.oltName}
            </div>
          </div>
          <span
            style={{
              color: statusSurfaces[signalTone(signal.rxPower)].accent,
              fontSize: 14,
              fontWeight: 600,
              fontVariantNumeric: "tabular-nums",
              whiteSpace: "nowrap",
            }}
          >
            {signal.rxPower.toFixed(1)} dBm
          </span>
        </div>
      ))}
    </div>
  );
}
