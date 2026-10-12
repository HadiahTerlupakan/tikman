import { useState } from "react";
import { message, type FormInstance } from "antd";
import type { RouterConnectionTestResult } from "@/domain/entities";
import { useTestRouterConnection } from "@/application/hooks/useMikrotikRouters";
import { confirmedFingerprint, requestErrorText } from "./routerTestSteps";

export interface RouterFormValues {
  name: string;
  host: string;
  snmpCommunity: string;
  snmpPort: number;
  apiUsername: string;
  apiPassword: string;
  apiPort: number;
}

/**
 * The two-step connection test of a new router: step 1 reads the certificate,
 * step 2 sends the REST password only to the fingerprint step 1 showed.
 */
export function useRouterTestSteps(form: FormInstance<RouterFormValues>) {
  const test = useTestRouterConnection();
  const [first, setFirst] = useState<RouterConnectionTestResult | null>(null);
  const [second, setSecond] = useState<RouterConnectionTestResult | null>(null);

  const runFirstStep = async () => {
    try {
      setSecond(null);
      setFirst(await test.mutateAsync(await form.validateFields()));
    } catch (error) {
      message.error(requestErrorText(error));
    }
  };

  const runSecondStep = async () => {
    try {
      const values = await form.validateFields();
      setSecond(
        await test.mutateAsync({
          ...values,
          tlsFingerprint: first?.fingerprint,
        }),
      );
    } catch (error) {
      message.error(requestErrorText(error));
    }
  };

  const reset = () => {
    setFirst(null);
    setSecond(null);
  };

  return {
    first,
    second,
    fingerprint: confirmedFingerprint(first, second),
    testing: test.isPending,
    runFirstStep,
    runSecondStep,
    reset,
  };
}
