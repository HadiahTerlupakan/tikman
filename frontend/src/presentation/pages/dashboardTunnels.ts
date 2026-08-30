import type { Olt, WireguardPeer } from "@/domain/entities";
import { formatAge } from "./dashboardStats";

export type TunnelState = "down" | "never" | "connected" | "disabled";

export interface TunnelRow {
  id: string;
  name: string;
  state: TunnelState;
  /** The evidence behind the state, so it is not a bare claim. */
  detail: string;
  /** OLTs this server can only reach through this tunnel. */
  oltCount: number;
}

export interface TunnelSummary {
  /** Tunnels expected to be up. A switched-off one is not a fault. */
  expected: number;
  connected: number;
  disabled: number;
  rows: TunnelRow[];
}

// A tunnel that has been down for hours, one that dropped a minute ago and one
// that was never set up all read as "not connected" but call for three
// different actions, so the row that needs a phone call sorts first.
const URGENCY: Record<TunnelState, number> = {
  down: 0,
  never: 1,
  connected: 2,
  disabled: 3,
};

export function summariseTunnels(
  peers: WireguardPeer[] | undefined,
  olts: Olt[] | undefined,
  now: number,
): TunnelSummary {
  const list = Array.isArray(peers) ? peers : [];
  const enabled = list.filter((peer) => peer.enabled);

  const rows = list
    .map((peer) => {
      const { state, detail } = describeTunnelState(peer, now);
      return {
        id: peer.id,
        name: peer.name,
        state,
        detail,
        oltCount: oltsBehindTunnel(peer, olts),
      };
    })
    .sort(
      (a, b) =>
        URGENCY[a.state] - URGENCY[b.state] || a.name.localeCompare(b.name),
    );

  return {
    expected: enabled.length,
    connected: enabled.filter((peer) => peer.connected).length,
    disabled: list.length - enabled.length,
    rows,
  };
}

export function describeTunnelState(
  peer: WireguardPeer,
  now: number,
): { state: TunnelState; detail: string } {
  if (!peer.enabled) {
    return { state: "disabled", detail: "switched off" };
  }

  const age = peer.lastHandshakeAt
    ? formatAge(now - new Date(peer.lastHandshakeAt).getTime())
    : null;

  if (peer.connected) {
    // "Connected" is a claim the operator cannot check; a handshake age is the
    // proof that the tunnel is alive now rather than earlier.
    return {
      state: "connected",
      detail: age ? `handshake ${age}` : "handshake pending",
    };
  }

  if (!age) {
    return { state: "never", detail: "never connected" };
  }
  return { state: "down", detail: `last seen ${age}` };
}

/**
 * How many OLTs sit inside the subnets this tunnel carries. A dead tunnel with
 * nothing behind it is a note; one carrying two OLTs is an outage, and the rest
 * of the dashboard cannot tell the difference on its own.
 */
export function oltsBehindTunnel(
  peer: WireguardPeer,
  olts: Olt[] | undefined,
): number {
  const list = Array.isArray(olts) ? olts : [];
  // The subnets arrive over the network. A peer that reached the page without
  // them must not blank the whole dashboard on a dropped field.
  const subnets = Array.isArray(peer.allowedIps) ? peer.allowedIps : [];

  return list.filter((olt) =>
    subnets.some((subnet) => addressInSubnet(olt.ipAddress, subnet)),
  ).length;
}

// A stored subnet that no longer parses cannot route anything, so it matches
// nothing rather than throwing on a page the operator needs during an outage.
function addressInSubnet(address: string, subnet: string): boolean {
  const [base, prefixText] = subnet.split("/");
  const prefix = Number(prefixText);
  const baseNumber = ipToNumber(base ?? "");
  const addressNumber = ipToNumber(address ?? "");

  if (baseNumber === null || addressNumber === null) {
    return false;
  }
  if (!Number.isInteger(prefix) || prefix < 0 || prefix > 32) {
    return false;
  }
  if (prefix === 0) {
    return true;
  }

  const mask = (-1 << (32 - prefix)) >>> 0;
  return (baseNumber & mask) >>> 0 === (addressNumber & mask) >>> 0;
}

const OCTET = /^\d{1,3}$/;

function ipToNumber(ip: string): number | null {
  const parts = ip.trim().split(".");
  if (parts.length !== 4) {
    return null;
  }

  let value = 0;
  for (const part of parts) {
    if (!OCTET.test(part)) {
      return null;
    }
    const octet = Number(part);
    if (octet > 255) {
      return null;
    }
    value = value * 256 + octet;
  }
  return value;
}
