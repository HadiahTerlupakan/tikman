import type { WireguardPeer } from "@/domain/entities";

export type TunnelTone = "success" | "error" | "default";

export interface DescribedTunnel {
  tone: TunnelTone;
  label: string;
  hint: string;
}

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
      hint: "Tunnel dimatikan dari halaman ini. Aktifkan kembali untuk memantau site.",
    };
  }

  if (peer.connected) {
    return { tone: "success", label: "Terhubung", hint: "" };
  }

  if (!peer.lastHandshakeAt) {
    return {
      tone: "error",
      label: "Belum pernah terhubung",
      hint: "Salin konfigurasi ke perangkat di site, lalu pastikan port UDP server terbuka.",
    };
  }

  return {
    tone: "error",
    label: `Tidak terhubung sejak ${formatSince(peer.lastHandshakeAt, now)}`,
    hint: "Periksa koneksi internet site dan apakah perangkat di lokasi masih menyala.",
  };
}

function formatSince(timestamp: string, now: Date): string {
  const minutes = Math.floor(
    (now.getTime() - new Date(timestamp).getTime()) / MINUTE_MS,
  );
  if (minutes < 60) {
    return `${minutes} menit lalu`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours} jam lalu`;
  }
  return `${Math.floor(hours / 24)} hari lalu`;
}
