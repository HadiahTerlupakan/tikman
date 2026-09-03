import { Button, Tooltip } from "antd";
import {
  BellOutlined,
  CheckCircleOutlined,
  StopOutlined,
} from "@ant-design/icons";
import type { PushPermission } from "@/infrastructure/firebase/messaging";

interface PushOptInButtonProps {
  permission: PushPermission;
  requesting: boolean;
  onEnable: () => void;
}

/**
 * Never prompts on its own — permission is only ever asked for from this
 * explicit click. Browsers throttle auto-prompted permission requests, and a
 * request the CS never asked for is the fastest way to get "Block" clicked
 * once and never askable again.
 */
export function PushOptInButton({
  permission,
  requesting,
  onEnable,
}: PushOptInButtonProps) {
  if (permission === "granted") {
    return (
      <Button icon={<CheckCircleOutlined />} disabled>
        Notifikasi aktif
      </Button>
    );
  }

  if (permission === "denied") {
    return (
      <Tooltip title="Diblokir oleh browser — aktifkan lagi lewat pengaturan situs">
        <Button icon={<StopOutlined />} disabled>
          Notifikasi diblokir
        </Button>
      </Tooltip>
    );
  }

  if (permission === "unsupported") {
    return null;
  }

  return (
    <Button icon={<BellOutlined />} loading={requesting} onClick={onEnable}>
      Aktifkan notifikasi
    </Button>
  );
}
