import type { ReactNode } from "react";
import { Skeleton } from "antd";
import { colors } from "@/shared/theme";
import { DarkCard } from "../common";

interface SummaryCardProps {
  label: string;
  // null renders a dash: the query failed, so the real count is unknown.
  value: number | null;
  icon: ReactNode;
  isLoading?: boolean;
  caption?: string;
}

export function SummaryCard({
  label,
  value,
  icon,
  isLoading,
  caption,
}: SummaryCardProps) {
  return (
    <DarkCard style={{ height: "100%" }}>
      <div style={{ display: "flex", flexDirection: "column", gap: 10 }}>
        <span style={{ color: colors.textSecondary, fontSize: 13 }}>
          {label}
        </span>

        {isLoading ? (
          <Skeleton
            active
            title={{ width: 72 }}
            paragraph={false}
            style={{ marginTop: 4 }}
          />
        ) : (
          <div style={{ display: "flex", alignItems: "center", gap: 10 }}>
            <span style={{ color: colors.textMuted, fontSize: 16 }}>
              {icon}
            </span>
            <span
              style={{
                color: colors.textPrimary,
                fontSize: 26,
                fontWeight: 600,
                lineHeight: 1.1,
                fontVariantNumeric: "tabular-nums",
              }}
            >
              {value === null ? "—" : value}
            </span>
          </div>
        )}

        {/* Reserved even when empty so all four cards share a baseline. */}
        <span style={{ fontSize: 12, color: colors.textMuted, minHeight: 18 }}>
          {caption ?? ""}
        </span>
      </div>
    </DarkCard>
  );
}
