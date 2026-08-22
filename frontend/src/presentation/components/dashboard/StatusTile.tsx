import { useState } from "react";
import type { ReactNode } from "react";
import { colors, statusSurfaces } from "@/shared/theme";
import type { StatusSurface } from "@/shared/theme";

export type StatusTone = "success" | "danger" | "warning" | "neutral";

// Fault tones make a claim on attention, so they only keep it while there is
// something to act on. "Online" is exempt: zero online means a full outage.
const ALARM_TONES: StatusTone[] = ["danger", "warning"];

function resolveSurface(tone: StatusTone, value: number | null): StatusSurface {
  if (ALARM_TONES.includes(tone) && (value === null || value === 0)) {
    return "quiet";
  }
  return tone;
}

interface StatusTileProps {
  tone: StatusTone;
  label: string;
  // null means the value is unknown (a failed query), rendered as a dash so it
  // cannot be mistaken for a real zero.
  value: number | null;
  total?: number;
  hint?: ReactNode;
  icon?: ReactNode;
}

export function StatusTile({
  tone,
  label,
  value,
  total,
  hint,
  icon,
}: StatusTileProps) {
  const [hovered, setHovered] = useState(false);
  const surface = resolveSurface(tone, value);
  const palette = statusSurfaces[surface];

  const share =
    value !== null && total !== undefined && total > 0
      ? `${Math.round((value / total) * 100)}% of ${total}`
      : null;

  return (
    <div
      data-tone={surface}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        position: "relative",
        background: palette.bg,
        border: `1px solid ${hovered ? palette.accent : palette.border}`,
        borderRadius: 10,
        padding: "16px 18px",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        gap: 6,
        overflow: "hidden",
        transition: "border-color 200ms ease, background 200ms ease",
      }}
    >
      <span
        aria-hidden
        style={{
          position: "absolute",
          left: 0,
          top: 0,
          bottom: 0,
          width: 2,
          background: surface === "quiet" ? "transparent" : palette.accent,
        }}
      />
      <div
        style={{
          display: "flex",
          alignItems: "center",
          justifyContent: "space-between",
          gap: 8,
        }}
      >
        <span
          style={{
            color: colors.textSecondary,
            fontSize: 12,
            fontWeight: 500,
            letterSpacing: "0.04em",
            textTransform: "uppercase",
          }}
        >
          {label}
        </span>
        {icon && (
          <span style={{ color: palette.accent, fontSize: 14, opacity: 0.85 }}>
            {icon}
          </span>
        )}
      </div>
      <div
        style={{
          color: palette.accent,
          fontSize: 30,
          fontWeight: 600,
          lineHeight: 1.15,
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {value === null ? "—" : value}
      </div>
      <div style={{ fontSize: 12, color: palette.hint, minHeight: 18 }}>
        {hint ?? share ?? ""}
      </div>
    </div>
  );
}
