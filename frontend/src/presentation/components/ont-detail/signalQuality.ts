// Thresholds a GPON technician works to. Below -28 dBm a link is close to the
// receiver's limit; above -8 dBm the ONU is being overdriven.
const RX_GOOD_MIN = -25;
const RX_WARN_MIN = -28;
const RX_OVERLOAD = -8;

// A healthy ONU transmits between 0 and 4 dBm.
const TX_MIN = 0;
const TX_MAX = 4;

export function rxSignalQuality(power: number): {
  label: string;
  color: string;
} {
  if (power > RX_OVERLOAD) return { label: "Too strong", color: "warning" };
  if (power >= RX_GOOD_MIN) return { label: "Good", color: "success" };
  if (power >= RX_WARN_MIN) return { label: "Marginal", color: "warning" };
  return { label: "Weak", color: "error" };
}

export function txSignalQuality(power: number): {
  label: string;
  color: string;
} {
  if (power >= TX_MIN && power <= TX_MAX) {
    return { label: "Normal", color: "success" };
  }
  return { label: "Out of range", color: "error" };
}
