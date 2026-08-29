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

  it("backs up 'Terhubung' with how recent the handshake was", () => {
    // "Terhubung" on its own is a claim the operator cannot check; the age of
    // the handshake is the evidence that the tunnel is alive now.
    const described = describeTunnel(
      peer({ connected: true, lastHandshakeAt: "2026-08-29T09:59:30Z" }),
      now,
    );

    expect(described.detail).toBe("handshake 30 detik lalu");
  });

  it("counts in seconds, so a fresh handshake does not read as zero minutes", () => {
    const described = describeTunnel(
      peer({ connected: false, lastHandshakeAt: "2026-08-29T09:59:20Z" }),
      now,
    );

    expect(described.label).toBe("Tidak terhubung sejak 40 detik lalu");
    expect(described.label).not.toContain("0 menit");
  });

  it("leaves the detail empty where the label already carries the timing", () => {
    for (const state of [
      peer({}),
      peer({ enabled: false }),
      peer({ connected: false, lastHandshakeAt: "2026-08-29T09:48:00Z" }),
    ]) {
      expect(describeTunnel(state, now).detail).toBe("");
    }
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
