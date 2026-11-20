import type { SwitchProtocol, SwitchVendor } from "@/domain/entities";

export const TELNET_WARNING =
  "Telnet tidak terenkripsi; pakai hanya lewat VPN TikMan atau jaringan manajemen.";

// The first protocol is the default: SSH where the vendor has it, since telnet
// sends the password in clear.
const VENDOR_PROTOCOLS: Record<SwitchVendor, SwitchProtocol[]> = {
  huawei_ce: ["ssh", "telnet"],
  fiberhome: ["ssh", "telnet"],
  crs: ["rest"],
};

export const PROTOCOL_LABEL: Record<SwitchProtocol, string> = {
  ssh: "SSH",
  telnet: "Telnet",
  rest: "REST",
};

export const PROTOCOL_DEFAULT_PORT: Record<SwitchProtocol, number> = {
  ssh: 22,
  telnet: 23,
  rest: 443,
};

/** Only Huawei CE has a driver so far; the others are shown as coming. */
export const VENDOR_OPTIONS: {
  label: string;
  value: SwitchVendor;
  disabled: boolean;
}[] = [
  { label: "Huawei CE", value: "huawei_ce", disabled: false },
  { label: "MikroTik CRS (segera)", value: "crs", disabled: true },
  { label: "FiberHome (segera)", value: "fiberhome", disabled: true },
];

export function protocolsFor(vendor: SwitchVendor): SwitchProtocol[] {
  return VENDOR_PROTOCOLS[vendor];
}
