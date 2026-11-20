import { Alert, Button, Typography } from "antd";
import type { ManagedSwitch } from "@/domain/entities";
import { useTrustSwitchHostKey } from "@/application/hooks/useSwitches";
import { ApiError } from "@/infrastructure/http/errorMapper";

const HTTP_CONFLICT = 409;

const CHANGED =
  "Kunci SSH switch berubah. Login dihentikan sampai kunci baru dipercaya.";
const UNPINNED =
  "Kunci SSH switch belum dipercaya. Inventaris dan backup tidak berjalan sampai kunci ini dipercaya.";

function trustError(error: unknown): string {
  if (error instanceof ApiError && error.statusCode === HTTP_CONFLICT) {
    return "Switch kini menyodorkan sidik jari lain. Muat ulang halaman dan periksa sidik jarinya sebelum mempercayai.";
  }
  return "Kunci gagal dipercaya. Coba lagi.";
}

/**
 * Shown while the poller is kept out of an SSH switch: its key changed, or it
 * presented one nobody has pinned yet (a switch added without the form's
 * test, or edited from telnet to SSH). Trusting sends the fingerprint shown
 * here, so a key that changed after the page loaded is refused.
 */
export function SwitchHostKeyBanner(props: {
  sw: ManagedSwitch;
  isAdmin: boolean;
}) {
  const { sw, isAdmin } = props;
  const trust = useTrustSwitchHostKey();
  const presented = sw.presentedFingerprint;
  if (sw.protocol !== "ssh" || !presented) return null;
  const unpinned = !sw.hostKeyChanged && presented !== sw.hostKeyFingerprint;
  if (!sw.hostKeyChanged && !unpinned) return null;
  return (
    <Alert
      type={sw.hostKeyChanged ? "error" : "warning"}
      showIcon
      style={{ marginBottom: 16 }}
      message={sw.hostKeyChanged ? CHANGED : UNPINNED}
      description={
        <>
          <div>Sidik jari yang disodorkan switch:</div>
          <Typography.Text code copyable>
            {presented}
          </Typography.Text>
          {trust.error && (
            <div>
              <Typography.Text type="danger">
                {trustError(trust.error)}
              </Typography.Text>
            </div>
          )}
        </>
      }
      action={
        isAdmin && (
          <Button
            danger
            loading={trust.isPending}
            onClick={() => trust.mutate({ id: sw.id, fingerprint: presented })}
          >
            Percayai
          </Button>
        )
      }
    />
  );
}
