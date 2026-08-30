import { Skeleton } from "antd";
import type { Olt, WireguardPeer } from "@/domain/entities";
import { colors, statusSurfaces } from "@/shared/theme";
import type {
  TunnelRow,
  TunnelState,
} from "@/presentation/pages/dashboardTunnels";
import { summariseTunnels } from "@/presentation/pages/dashboardTunnels";
import { DarkCard } from "../common";

interface VpnStatusCardProps {
  peers: WireguardPeer[] | undefined;
  olts: Olt[] | undefined;
  isLoading?: boolean;
  isError?: boolean;
}

// Enough rows to act on without the card turning into the VPN page.
const ROWS_SHOWN = 6;

const STATE_SURFACE: Record<TunnelState, keyof typeof statusSurfaces> = {
  down: "danger",
  never: "warning",
  connected: "success",
  disabled: "quiet",
};

/**
 * Tunnel reachability for the sites this server polls. An OLT that stopped
 * answering because its site's tunnel dropped looks identical to a dead OLT on
 * the rest of this page; this card is what tells the two apart.
 */
export function VpnStatusCard({
  peers,
  olts,
  isLoading,
  isError,
}: VpnStatusCardProps) {
  const summary = summariseTunnels(peers, olts, Date.now());
  const hidden = summary.rows.length - ROWS_SHOWN;

  return (
    <DarkCard title="Site Tunnels" style={{ height: "100%" }}>
      {isLoading ? (
        <Skeleton active paragraph={{ rows: 3 }} title={false} />
      ) : isError ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          Tunnel status could not be loaded.
        </div>
      ) : summary.rows.length === 0 ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          No site tunnels configured.
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 14 }}>
          <Headline summary={summary} />

          <div style={{ display: "flex", flexDirection: "column" }}>
            {summary.rows.slice(0, ROWS_SHOWN).map((row, index) => (
              <TunnelLine key={row.id} row={row} isFirst={index === 0} />
            ))}
          </div>

          {hidden > 0 && (
            <span style={{ color: colors.textMuted, fontSize: 11 }}>
              +{hidden} more on the VPN page
            </span>
          )}
        </div>
      )}
    </DarkCard>
  );
}

function Headline({
  summary,
}: {
  summary: ReturnType<typeof summariseTunnels>;
}) {
  if (summary.expected === 0) {
    return (
      <span style={{ color: colors.textSecondary, fontSize: 13 }}>
        Every site tunnel is switched off.
      </span>
    );
  }

  const allUp = summary.connected === summary.expected;

  return (
    <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
      <span
        style={{
          color: allUp
            ? statusSurfaces.success.accent
            : statusSurfaces.danger.accent,
          fontSize: 26,
          fontWeight: 600,
          lineHeight: 1.1,
          fontVariantNumeric: "tabular-nums",
        }}
      >
        {summary.connected}
      </span>
      <span style={{ color: colors.textSecondary, fontSize: 13 }}>
        of {summary.expected} sites connected
      </span>
    </div>
  );
}

function TunnelLine({ row, isFirst }: { row: TunnelRow; isFirst: boolean }) {
  const surface = statusSurfaces[STATE_SURFACE[row.state]];

  return (
    <div
      style={{
        display: "flex",
        alignItems: "center",
        justifyContent: "space-between",
        gap: 10,
        padding: "8px 0",
        borderTop: isFirst ? "none" : `1px solid ${colors.border}`,
      }}
    >
      <div
        style={{ display: "flex", alignItems: "center", gap: 8, minWidth: 0 }}
      >
        <span
          aria-hidden
          style={{
            width: 8,
            height: 8,
            borderRadius: 4,
            background: surface.accent,
            flexShrink: 0,
          }}
        />
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
            {row.name}
          </div>
          <div style={{ color: surface.hint, fontSize: 11 }}>{row.detail}</div>
        </div>
      </div>

      {/* Silent when nothing is behind the tunnel: a zero would read as a fault
          rather than as "this outage reaches no hardware". */}
      {row.oltCount > 0 && (
        <span
          style={{
            color: colors.textMuted,
            fontSize: 11,
            whiteSpace: "nowrap",
          }}
        >
          {row.oltCount} OLT{row.oltCount > 1 ? "s" : ""}
        </span>
      )}
    </div>
  );
}
