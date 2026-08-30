import { describe, expect, it } from "vitest";
import type { Olt, WireguardPeer } from "@/domain/entities";
import {
  describeTunnelState,
  oltsBehindTunnel,
  summariseTunnels,
} from "./dashboardTunnels";

const NOW = new Date("2026-08-30T12:00:00Z").getTime();
const minutesAgo = (n: number) => new Date(NOW - n * 60_000).toISOString();

const peer = (overrides: Partial<WireguardPeer>): WireguardPeer =>
  ({
    id: "p1",
    name: "Depok",
    allowedIps: [],
    enabled: true,
    connected: true,
    lastHandshakeAt: minutesAgo(0.2),
    ...overrides,
  }) as WireguardPeer;

const olt = (ipAddress: string): Olt => ({ ipAddress }) as Olt;

describe("describeTunnelState", () => {
  it("backs a connected tunnel with the age of its handshake", () => {
    // "Connected" alone is a claim the operator has no way to check.
    const { state, detail } = describeTunnelState(
      peer({ lastHandshakeAt: minutesAgo(0.2) }),
      NOW,
    );

    expect(state).toBe("connected");
    expect(detail).toBe("handshake 12s ago");
  });

  it("separates a site that dropped from one that was never set up", () => {
    // The first needs a phone call to the site; the second needs the config
    // pasting into its router. Both read as "not connected".
    expect(
      describeTunnelState(
        peer({ connected: false, lastHandshakeAt: minutesAgo(360) }),
        NOW,
      ),
    ).toEqual({ state: "down", detail: "last seen 6h ago" });

    expect(
      describeTunnelState(
        peer({ connected: false, lastHandshakeAt: null }),
        NOW,
      ),
    ).toEqual({ state: "never", detail: "never connected" });
  });

  it("does not call a deliberately disabled tunnel a fault", () => {
    expect(
      describeTunnelState(peer({ enabled: false, connected: false }), NOW),
    ).toEqual({ state: "disabled", detail: "switched off" });
  });
});

describe("oltsBehindTunnel", () => {
  it("counts the OLTs inside the subnets the tunnel carries", () => {
    const count = oltsBehindTunnel(
      peer({ allowedIps: ["192.168.220.0/24", "10.20.0.0/16"] }),
      [
        olt("192.168.220.22"),
        olt("10.20.5.1"),
        olt("192.168.221.5"),
        olt("8.8.8.8"),
      ],
    );

    expect(count).toBe(2);
  });

  it("respects the prefix rather than matching on the leading octets", () => {
    expect(
      oltsBehindTunnel(peer({ allowedIps: ["192.168.220.0/25"] }), [
        olt("192.168.220.10"),
        olt("192.168.220.200"),
      ]),
    ).toBe(1);
  });

  it("matches nothing for a subnet that no longer parses", () => {
    // A broken stored value cannot route traffic, and this card is read during
    // an outage — it must not be the thing that throws.
    expect(
      oltsBehindTunnel(peer({ allowedIps: ["not-a-subnet", "10.0.0.0/99"] }), [
        olt("10.0.0.1"),
      ]),
    ).toBe(0);
  });

  it("is zero when the tunnel carries no subnet at all", () => {
    expect(oltsBehindTunnel(peer({ allowedIps: [] }), [olt("10.0.0.1")])).toBe(
      0,
    );
  });

  it("survives a peer that arrived without its subnets", () => {
    // This card is read during an outage. A dropped field must not be what
    // blanks the page.
    const withoutSubnets = peer({}) as unknown as Record<string, unknown>;
    delete withoutSubnets.allowedIps;

    expect(oltsBehindTunnel(withoutSubnets as never, [olt("10.0.0.1")])).toBe(
      0,
    );
  });
});

describe("summariseTunnels", () => {
  it("counts connected sites against the ones expected to be up", () => {
    const summary = summariseTunnels(
      [
        peer({ id: "1", name: "Depok" }),
        peer({ id: "2", name: "Bekasi", connected: false }),
        peer({ id: "3", name: "Lama", enabled: false, connected: false }),
      ],
      [],
      NOW,
    );

    expect(summary.connected).toBe(1);
    expect(summary.expected).toBe(2);
    expect(summary.disabled).toBe(1);
  });

  it("puts the tunnel that needs a phone call first and the disabled one last", () => {
    const summary = summariseTunnels(
      [
        peer({ id: "1", name: "Healthy" }),
        peer({ id: "2", name: "Off", enabled: false, connected: false }),
        peer({ id: "3", name: "New", connected: false, lastHandshakeAt: null }),
        peer({
          id: "4",
          name: "Dropped",
          connected: false,
          lastHandshakeAt: minutesAgo(120),
        }),
      ],
      [],
      NOW,
    );

    expect(summary.rows.map((r) => r.name)).toEqual([
      "Dropped",
      "New",
      "Healthy",
      "Off",
    ]);
  });

  it("carries how much hardware each tunnel is responsible for", () => {
    const summary = summariseTunnels(
      [peer({ id: "1", allowedIps: ["192.168.220.0/24"] })],
      [olt("192.168.220.22"), olt("192.168.220.23")],
      NOW,
    );

    expect(summary.rows[0].oltCount).toBe(2);
  });

  it("returns an empty summary rather than throwing on no data", () => {
    expect(summariseTunnels(undefined, undefined, NOW)).toEqual({
      expected: 0,
      connected: 0,
      disabled: 0,
      rows: [],
    });
  });
});
