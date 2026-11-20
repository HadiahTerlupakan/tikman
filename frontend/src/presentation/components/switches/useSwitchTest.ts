import { useState } from "react";
import type { FormInstance } from "antd";
import type {
  CreateSwitchDto,
  SwitchConnectionTestResult,
} from "@/domain/entities";
import { useTestSwitchConnection } from "@/application/hooks/useSwitches";

export interface SwitchFormValues extends CreateSwitchDto {
  rackPosition?: number | null;
}

export function errorText(error: unknown): string {
  return error instanceof Error && error.message
    ? error.message
    : "Permintaan gagal";
}

/**
 * The connection test of a new switch. Over SSH the first test only reads the
 * host key; "Percayai kunci ini" tests again pinned to that key, and the
 * password is sent only then. Telnet has no key, so one test is all of it.
 */
export function useSwitchTest(form: FormInstance<SwitchFormValues>) {
  const test = useTestSwitchConnection();
  const [result, setResult] = useState<SwitchConnectionTestResult | null>(null);
  const [trusted, setTrusted] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);

  const run = async (hostKeyFingerprint?: string) => {
    setError(null);
    setResult(null);
    try {
      const values = await form.validateFields();
      setResult(await test.mutateAsync({ ...values, hostKeyFingerprint }));
    } catch (e) {
      // A form that fails validation already marks its own fields.
      if (!(e as { errorFields?: unknown }).errorFields) setError(errorText(e));
    }
  };

  const trust = (fingerprint: string) => {
    setTrusted(fingerprint);
    return run(fingerprint);
  };

  const reset = () => {
    setResult(null);
    setTrusted(null);
    setError(null);
  };

  // What a new switch may be saved with: null until a test identified it,
  // "" for no key (telnet), or the key the passing test was pinned to.
  const savable = result?.kind === "identified" ? trusted ?? "" : null;

  return {
    result,
    error,
    savable,
    testing: test.isPending,
    run: () => run(trusted ?? undefined),
    trust,
    reset,
  };
}
