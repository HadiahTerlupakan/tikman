import type { PeerConfigFormat } from "@/domain/entities";

export interface ClientSteps {
  /** What the text below the steps actually is, so it is not pasted wrongly. */
  intro: string;
  steps: string[];
  /** How the operator confirms it worked, without leaving the device. */
  verify: string;
}

// These sit next to the generated config rather than only in the documentation:
// the operator reads them while standing at the device, and the two formats are
// used differently — one is a block of commands, the other is a file.
const MIKROTIK: ClientSteps = {
  intro: "Teks di bawah adalah perintah RouterOS, dijalankan sekali.",
  steps: [
    "Buka Winbox atau SSH ke MikroTik di lokasi tersebut.",
    "Masuk ke menu New Terminal.",
    "Tempel seluruh blok di bawah sekaligus, lalu tekan Enter.",
  ],
  verify:
    "Periksa dengan: /interface/wireguard/peers print — kolom last-handshake akan terisi dalam beberapa detik.",
};

const WG_QUICK: ClientSteps = {
  intro: "Teks di bawah adalah isi berkas konfigurasi, bukan perintah.",
  steps: [
    "Pasang WireGuard bila belum ada: sudo apt install wireguard",
    "Simpan isi di bawah sebagai /etc/wireguard/wg0.conf",
    "Batasi izinnya: sudo chmod 600 /etc/wireguard/wg0.conf",
    "Nyalakan: sudo wg-quick up wg0",
    "Aktifkan saat boot: sudo systemctl enable wg-quick@wg0",
  ],
  verify:
    "Periksa dengan: sudo wg show — baris latest handshake akan muncul dalam beberapa detik.",
};

export function clientSteps(format: PeerConfigFormat): ClientSteps {
  return format === "mikrotik" ? MIKROTIK : WG_QUICK;
}
