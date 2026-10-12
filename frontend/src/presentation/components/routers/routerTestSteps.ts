import type { AxiosError } from "axios";
import type { RouterConnectionTestResult } from "@/domain/entities";

export function requestErrorText(error: unknown): string {
  return (
    (error as AxiosError<{ error?: string }>).response?.data?.error ??
    "Permintaan gagal"
  );
}

/**
 * The backend only sends REST credentials to a certificate the operator has
 * confirmed, so a router may be saved only once step 2 (run against the
 * step-1 fingerprint) reached both SNMP and REST. Returns that fingerprint,
 * or null while saving is not yet safe.
 */
export function confirmedFingerprint(
  first: RouterConnectionTestResult | null,
  second: RouterConnectionTestResult | null,
): string | null {
  if (!first?.fingerprint || first.snmpError) return null;
  if (!second || second.snmpError || second.apiError) return null;
  return first.fingerprint;
}
