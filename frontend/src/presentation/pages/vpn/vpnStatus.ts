import type { WireguardPeer } from "@/domain/entities";

export type TunnelTone = "success" | "error" | "default";

export interface DescribedTunnel {
  tone: TunnelTone;
  label: string;
  /** Shown under the label. Carries the evidence behind a bare "Terhubung". */
  detail: string;
  hint: string;
}

const SECOND_MS = 1_000;
const MINUTE_MS = 60_000;

// The operator should not have to read a handshake timestamp to know whether a
// site is up, so every state carries its own next step.
export function describeTunnel(
  peer: WireguardPeer,
  now: Date,
): DescribedTunnel {
  if (!peer.enabled) {
    return {
      tone: "default",
      label: "Dinonaktifkan",
      detail: "",
      hint: "Tunnel dimatikan dari halaman ini. Aktifkan kembali untuk memantau site.",
    };
  }

  if (peer.connected) {
    return {
      tone: "success",
      label: "Terhubung",
      // Without this, "Terhubung" is a claim the operator cannot check. A
      // handshake age proves the tunnel is alive now rather than earlier.
      detail: peer.lastHandshakeAt
        ? `handshake ${formatSince(peer.lastHandshakeAt, now)}`
        : "",
      hint: "",
    };
  }

  if (!peer.lastHandshakeAt) {
    return {
      tone: "error",
      label: "Belum pernah terhubung",
      detail: "",
      hint: "Salin konfigurasi ke perangkat di site, lalu pastikan port UDP server terbuka.",
    };
  }

  return {
    tone: "error",
    label: `Tidak terhubung sejak ${formatSince(peer.lastHandshakeAt, now)}`,
    detail: "",
    hint: "Periksa koneksi internet site dan apakah perangkat di lokasi masih menyala.",
  };
}

function formatSince(timestamp: string, now: Date): string {
  const elapsed = now.getTime() - new Date(timestamp).getTime();

  // Seconds matter here: a tunnel that just came up would otherwise read
  // "0 menit lalu", which looks like a fault rather than a fresh handshake.
  const seconds = Math.floor(elapsed / SECOND_MS);
  if (seconds < 60) {
    return `${Math.max(seconds, 0)} detik lalu`;
  }

  const minutes = Math.floor(elapsed / MINUTE_MS);
  if (minutes < 60) {
    return `${minutes} menit lalu`;
  }

  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours} jam lalu`;
  }
  return `${Math.floor(hours / 24)} hari lalu`;
}
