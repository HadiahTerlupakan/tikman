import { Skeleton } from "antd";
import { colors } from "@/shared/theme";
import { DarkCard } from "../common";
import type { DependencyStatus, Health } from "@/domain/entities";

const LABELS: Record<DependencyStatus, string> = {
  up: "Connected",
  down: "Unreachable",
  disabled: "Not configured",
  unknown: "Unknown",
};

// A 503 means the API answered but a dependency it needs did not, which is a
// different situation from the API being unreachable — the rows must not both
// claim "Unreachable" or the operator cannot tell which to investigate.
const API_LABELS: Record<DependencyStatus, string> = {
  up: "Healthy",
  down: "Unreachable",
  disabled: "Not configured",
  unknown: "Unknown",
};

// Only a fault earns a tinted row; a healthy dependency is the expected state
// and stays on the plain card surface.
const TONE: Record<
  DependencyStatus,
  { dot: string; text: string; bg: string; border: string }
> = {
  up: {
    dot: colors.success,
    text: colors.success,
    bg: "transparent",
    border: colors.border,
  },
  down: {
    dot: colors.danger,
    text: colors.danger,
    bg: "rgba(248, 113, 113, 0.09)",
    border: "rgba(248, 113, 113, 0.32)",
  },
  disabled: {
    dot: colors.textMuted,
    text: colors.textMuted,
    bg: "transparent",
    border: colors.border,
  },
  unknown: {
    dot: colors.warning,
    text: colors.warning,
    bg: "rgba(251, 191, 36, 0.09)",
    border: "rgba(251, 191, 36, 0.30)",
  },
};

interface SystemHealthCardProps {
  health?: Health;
  isLoading: boolean;
}

export function SystemHealthCard({ health, isLoading }: SystemHealthCardProps) {
  if (isLoading || !health?.dependencies) {
    return (
      <DarkCard title="System Health">
        <Skeleton active={isLoading} paragraph={{ rows: 3 }} title={false} />
      </DarkCard>
    );
  }

  // Only a request that never landed means the API is down; a degraded response
  // proves the API is serving and points at the dependency instead.
  const apiStatus: DependencyStatus =
    health.status === "unreachable" ? "down" : "up";
  const apiLabel =
    health.status === "degraded" ? "Degraded" : API_LABELS[apiStatus];

  const rows: Array<{
    label: string;
    status: DependencyStatus;
    text: string;
  }> = [
    {
      label: "API Server",
      status: health.status === "degraded" ? "unknown" : apiStatus,
      text: apiLabel,
    },
    {
      label: "Database",
      status: health.dependencies.database,
      text: LABELS[health.dependencies.database],
    },
    {
      label: "Redis Cache",
      status: health.dependencies.redis,
      text: LABELS[health.dependencies.redis],
    },
  ];

  return (
    <DarkCard title="System Health">
      <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
        {rows.map((row) => (
          <div
            key={row.label}
            style={{
              padding: "11px 14px",
              background: TONE[row.status].bg,
              border: `1px solid ${TONE[row.status].border}`,
              borderRadius: 8,
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <div style={{ display: "flex", alignItems: "center", gap: 8 }}>
              <span
                style={{
                  width: 8,
                  height: 8,
                  backgroundColor: TONE[row.status].dot,
                  borderRadius: "50%",
                }}
              />
              <span style={{ color: colors.textBody }}>{row.label}</span>
            </div>
            <span style={{ color: TONE[row.status].text, fontWeight: 500 }}>
              {row.text}
            </span>
          </div>
        ))}
      </div>
    </DarkCard>
  );
}
