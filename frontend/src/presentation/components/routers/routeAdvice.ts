import type {
  RouteAdvice,
  RouteAdviceStatus,
  RouteInbound,
} from "@/domain/entities";

export const ROUTE_STATUS: Record<
  RouteAdviceStatus,
  { color: string; label: string }
> = {
  transit_only: { color: "red", label: "Hanya lewat transit" },
  inbound_via_transit: {
    color: "orange",
    label: "Masuk lewat transit",
  },
  shorter_outbound: {
    color: "gold",
    label: "Jalur keluar bisa lebih pendek",
  },
  optimal: { color: "green", label: "Optimal" },
};

export function needsAction(advice: RouteAdvice): boolean {
  return advice.status !== "optimal";
}

/** The peer most of the download arrived from, with its share. */
export function mainInbound(
  inbound: RouteInbound[] | null,
): { peer: string; share: number } | null {
  if (!inbound || inbound.length === 0) return null;
  const total = inbound.reduce((sum, i) => sum + i.bytes, 0);
  if (total === 0) return null;
  const top = inbound.reduce((a, b) => (b.bytes > a.bytes ? b : a));
  return { peer: top.peer, share: top.bytes / total };
}

/** What to do about one prefix, in words; the draft carries any commands. */
export function adviceText(advice: RouteAdvice): string {
  const who = advice.name
    ? `${advice.name} (AS${advice.asn})`
    : `AS${advice.asn}`;
  const ix = (advice.alternatives ?? [])
    .concat(advice.active ? [advice.active] : [])
    .find((p) => !p.transit);
  const alt = (advice.alternatives ?? []).find(
    (p) => p.peer === advice.suggested,
  );
  switch (advice.status) {
    case "transit_only":
      return `Router hanya menerima rute ke prefix ini dari transit. Konfigurasi tidak bisa menambah jalur yang tidak diterima: pertimbangkan peering langsung atau cache dengan ${who}.`;
    case "inbound_via_transit":
      return `${who} terlihat lewat ${ix?.peer ?? "IX/CDN"}, tetapi download-nya sebagian besar masuk lewat transit. Local-pref di router ini hanya mengatur trafik keluar, jadi tidak bisa memindahkan download. Cek apakah ${who} menerima prefix Anda di ${ix?.peer ?? "IX tersebut"}, atau hubungi NOC mereka.`;
    case "shorter_outbound":
      return `Lewat ${alt?.peer} AS path-nya ${alt?.pathLength}, lebih pendek dari jalur aktif lewat ${advice.active?.peer} (${advice.active?.pathLength}). Draf di bawah menaikkan local-pref untuk cakupan yang Anda pilih dan hanya memindahkan trafik keluar (upload). Tinjau dulu sebelum dijalankan di router.`;
    default:
      return "Jalur keluar sudah yang terpendek di antara peer non-transit. Draf di bawah tetap bisa dipakai kalau Anda ingin mengutamakan peer lain.";
  }
}
