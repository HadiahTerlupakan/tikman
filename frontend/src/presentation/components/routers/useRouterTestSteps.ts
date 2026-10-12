import { useState } from "react";
import { message, type FormInstance } from "antd";
import type {
  RouterApiProtocol,
  RouterConnectionTestResult,
} from "@/domain/entities";
import { useTestRouterConnection } from "@/application/hooks/useMikrotikRouters";
import { requestErrorText, savableFingerprint } from "./routerTestSteps";

export interface RouterFormValues {
  name: string;
  host: string;
  snmpCommunity: string;
  snmpPort: number;
  apiUsername: string;
  apiPassword: string;
  apiPort: number;
  apiProtocol: RouterApiProtocol;
  flowEnabled?: boolean;
  flowSampling?: number;
}

/**
 * The connection test of a new router: over TLS, step 1 reads the certificate
 * and step 2 sends the password only to the fingerprint step 1 showed; plain
 * API is a single step.
 */
export function useRouterTestSteps(
  form: FormInstance<RouterFormValues>,
  protocol: RouterApiProtocol,
) {
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
    fingerprint: savableFingerprint(protocol, first, second),
    testing: test.isPending,
    runFirstStep,
    runSecondStep,
    reset,
  };
}
