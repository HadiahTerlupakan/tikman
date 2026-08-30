import { Skeleton } from "antd";
import { colors, statusSurfaces } from "@/shared/theme";

// The surfaces a segment may claim. "quiet" is deliberately absent: a bar
// segment with no colour would be invisible rather than restrained.
export type StatusTone = "success" | "danger" | "warning" | "neutral";

export interface StatusSegment {
  label: string;
  tone: StatusTone;
  // null means the figure is unknown (a failed query), never a real zero.
  value: number | null;
  hint?: string;
}

interface StatusBarProps {
  segments: StatusSegment[];
  total: number;
  isLoading?: boolean;
  emptyText: string;
}

/**
 * A proportional bar over a legend of counts. It replaces a row of equal-sized
 * tiles so the split is readable before any number is: a bar that is almost all
 * green needs no arithmetic to interpret.
 */
export function StatusBar({
  segments,
  total,
  isLoading,
  emptyText,
}: StatusBarProps) {
  if (isLoading) {
    return <Skeleton active paragraph={{ rows: 2 }} title={false} />;
  }

  if (total === 0) {
    return (
      <div style={{ color: colors.textSecondary, fontSize: 13, padding: 8 }}>
        {emptyText}
      </div>
    );
  }

  // Segments with nothing in them are dropped from the bar but kept in the
  // legend: a zero is worth reading, a zero-width sliver is not.
  const filled = segments.filter((s) => (s.value ?? 0) > 0);

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 16 }}>
      <div
        role="img"
        aria-label={segments
          .map((s) => `${s.label} ${s.value ?? "unknown"}`)
          .join(", ")}
        style={{
          display: "flex",
          height: 8,
          borderRadius: 4,
          overflow: "hidden",
          background: "rgba(161, 161, 170, 0.14)",
          gap: 2,
        }}
      >
        {filled.map((segment) => (
          <div
            key={segment.label}
            style={{
              flexGrow: segment.value ?? 0,
              background: statusSurfaces[segment.tone].accent,
            }}
          />
        ))}
      </div>

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(120px, 1fr))",
          gap: 16,
        }}
      >
        {segments.map((segment) => (
          <Legend key={segment.label} segment={segment} total={total} />
        ))}
      </div>
    </div>
  );
}

function Legend({ segment, total }: { segment: StatusSegment; total: number }) {
  const accent = statusSurfaces[segment.tone].accent;
  const share =
    segment.value !== null && total > 0
      ? `${Math.round((segment.value / total) * 100)}%`
      : null;

  return (
    <div style={{ display: "flex", flexDirection: "column", gap: 4 }}>
      <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
        <span
          aria-hidden
          style={{
            width: 8,
            height: 8,
            borderRadius: 2,
            background: accent,
            flexShrink: 0,
          }}
        />
        <span
          style={{
            color: colors.textSecondary,
            fontSize: 12,
            letterSpacing: "0.04em",
            textTransform: "uppercase",
          }}
        >
          {segment.label}
        </span>
      </div>
      <div style={{ display: "flex", alignItems: "baseline", gap: 8 }}>
        <span
          style={{
            color: colors.textPrimary,
            fontSize: 22,
            fontWeight: 600,
            lineHeight: 1.1,
            fontVariantNumeric: "tabular-nums",
          }}
        >
          {segment.value === null ? "—" : segment.value}
        </span>
        {share && (
          <span style={{ color: colors.textMuted, fontSize: 12 }}>{share}</span>
        )}
      </div>
      {segment.hint && (
        <span style={{ color: colors.textMuted, fontSize: 11 }}>
          {segment.hint}
        </span>
      )}
    </div>
  );
}
