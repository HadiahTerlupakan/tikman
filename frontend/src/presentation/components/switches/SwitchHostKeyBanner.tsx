import { Alert, Button, Typography } from "antd";
import type { ManagedSwitch } from "@/domain/entities";
import { useTrustSwitchHostKey } from "@/application/hooks/useSwitches";

/** Shown while a pinned SSH host key no longer matches; the poller stays out until an admin trusts the new one. */
export function SwitchHostKeyBanner(props: {
  sw: ManagedSwitch;
  isAdmin: boolean;
}) {
  const { sw, isAdmin } = props;
  const trust = useTrustSwitchHostKey();
  if (!sw.hostKeyChanged) return null;
  return (
    <Alert
      type="error"
      showIcon
      style={{ marginBottom: 16 }}
      message="Kunci SSH switch berubah. Login dihentikan sampai kunci baru dipercaya."
      description={
        <>
          <div>Sidik jari yang disodorkan switch:</div>
          <Typography.Text code copyable>
            {sw.presentedFingerprint}
          </Typography.Text>
        </>
      }
      action={
        isAdmin && (
          <Button
            danger
            loading={trust.isPending}
            onClick={() => trust.mutate(sw.id)}
          >
            Percayai
          </Button>
        )
      }
    />
  );
}
