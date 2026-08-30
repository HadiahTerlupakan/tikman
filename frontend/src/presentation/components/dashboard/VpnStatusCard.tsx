import { Skeleton } from "antd";
import type { WireguardPeer } from "@/domain/entities";
import { colors, statusSurfaces } from "@/shared/theme";
import { DarkCard } from "../common";

interface VpnStatusCardProps {
  peers: WireguardPeer[] | undefined;
  isLoading?: boolean;
  isError?: boolean;
}

// Enough names to act on without the card turning into the VPN page.
const NAMES_SHOWN = 4;

/**
 * Tunnel reachability for the sites this server polls. An OLT that stops
 * answering because its site's tunnel dropped looks identical to a dead OLT on
 * the rest of this page; this card is what tells the two apart.
 */
export function VpnStatusCard({
  peers,
  isLoading,
  isError,
}: VpnStatusCardProps) {
  const enabled = (peers ?? []).filter((peer) => peer.enabled);
  const down = enabled.filter((peer) => !peer.connected);
  const disabledCount = (peers ?? []).length - enabled.length;

  return (
    <DarkCard title="Site Tunnels" style={{ height: "100%" }}>
      {isLoading ? (
        <Skeleton active paragraph={{ rows: 2 }} title={false} />
      ) : isError ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          Tunnel status could not be loaded.
        </div>
      ) : enabled.length === 0 ? (
        <div style={{ color: colors.textSecondary, fontSize: 13 }}>
          {disabledCount > 0
            ? "Every site tunnel is switched off."
            : "No site tunnels configured."}
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: 12 }}>
          <div style={{ display: "flex", alignItems: "baseline", gap: 10 }}>
            <span
              style={{
                color:
                  down.length === 0
                    ? statusSurfaces.success.accent
                    : statusSurfaces.danger.accent,
                fontSize: 26,
                fontWeight: 600,
                lineHeight: 1.1,
                fontVariantNumeric: "tabular-nums",
              }}
            >
              {enabled.length - down.length}
            </span>
            <span style={{ color: colors.textSecondary, fontSize: 13 }}>
              of {enabled.length} sites connected
            </span>
          </div>

          {down.length > 0 && (
            <div style={{ color: statusSurfaces.danger.hint, fontSize: 12 }}>
              Down:{" "}
              {down
                .slice(0, NAMES_SHOWN)
                .map((p) => p.name)
                .join(", ")}
              {down.length > NAMES_SHOWN &&
                ` +${down.length - NAMES_SHOWN} more`}
            </div>
          )}

          {disabledCount > 0 && (
            <div style={{ color: colors.textMuted, fontSize: 11 }}>
              {disabledCount} switched off, not counted
            </div>
          )}
        </div>
      )}
    </DarkCard>
  );
}
