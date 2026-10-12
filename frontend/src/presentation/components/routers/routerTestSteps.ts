import type { AxiosError } from "axios";
import type {
  RouterApiProtocol,
  RouterConnectionTestResult,
} from "@/domain/entities";
import { usesTls } from "./routerProtocol";

export function requestErrorText(error: unknown): string {
  return (
    (error as AxiosError<{ error?: string }>).response?.data?.error ??
    "Permintaan gagal"
  );
}

/**
 * What a new router may be saved with, or null while saving is not yet safe.
 * Over TLS the backend sends credentials only to a certificate the operator
 * confirmed, so step 2, run against the step-1 fingerprint, must have reached
 * both SNMP and the API. Plain API has no certificate: its single step is the
 * whole test, and the router is saved with no fingerprint ("").
 */
export function savableFingerprint(
  protocol: RouterApiProtocol,
  first: RouterConnectionTestResult | null,
  second: RouterConnectionTestResult | null,
): string | null {
  if (!first || first.snmpError) return null;
  if (!usesTls(protocol)) return first.apiError ? null : "";
  if (!first.fingerprint) return null;
  if (!second || second.snmpError || second.apiError) return null;
  return first.fingerprint;
}
