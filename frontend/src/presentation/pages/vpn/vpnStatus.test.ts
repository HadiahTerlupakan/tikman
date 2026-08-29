import { describe, expect, it } from "vitest";
import type { WireguardPeer } from "@/domain/entities";
import { describeTunnel } from "./vpnStatus";

const peer = (overrides: Partial<WireguardPeer>): WireguardPeer =>
  ({
    connected: false,
    enabled: true,
    lastHandshakeAt: null,
    ...overrides,
  }) as WireguardPeer;

const now = new Date("2026-08-29T10:00:00Z");

describe("describeTunnel", () => {
  it("reports a connected tunnel without asking the reader to interpret a timestamp", () => {
    const described = describeTunnel(
      peer({ connected: true, lastHandshakeAt: "2026-08-29T09:59:30Z" }),
      now,
    );

    expect(described.tone).toBe("success");
    expect(described.label).toBe("Terhubung");
  });

  it("says how long a tunnel has been down", () => {
    const described = describeTunnel(
      peer({ connected: false, lastHandshakeAt: "2026-08-29T09:48:00Z" }),
      now,
    );

    expect(described.tone).toBe("error");
    expect(described.label).toBe("Tidak terhubung sejak 12 menit lalu");
  });

  it("distinguishes a tunnel that has never connected", () => {
    const described = describeTunnel(peer({}), now);

    expect(described.label).toBe("Belum pernah terhubung");
    expect(described.hint).toContain("konfigurasi");
  });

  it("marks a peer the operator switched off rather than calling it broken", () => {
    const described = describeTunnel(peer({ enabled: false }), now);

    expect(described.tone).toBe("default");
    expect(described.label).toBe("Dinonaktifkan");
  });
});
