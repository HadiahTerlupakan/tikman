import { useState } from "react";
import {
  requestPushPermission,
  type PushPermission,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";

const pushRepository = new PushRepository();

interface UsePushNotificationsResult {
  permission: PushPermission;
  requesting: boolean;
  enable: () => Promise<void>;
}

/** Drives the CS Inbox "Aktifkan notifikasi" control. Registering the device
 * on the backend happens here — infrastructure/firebase/messaging.ts only
 * knows about the browser and Firebase side of push. */
export function usePushNotifications(): UsePushNotificationsResult {
  const [permission, setPermission] = useState<PushPermission>(
    typeof Notification === "undefined"
      ? "unsupported"
      : Notification.permission,
  );
  const [requesting, setRequesting] = useState(false);

  const enable = async () => {
    setRequesting(true);
    try {
      const result = await requestPushPermission();
      setPermission(result.permission);
      if (result.token) {
        await pushRepository.subscribe(result.token);
      }
    } finally {
      setRequesting(false);
    }
  };

  return { permission, requesting, enable };
}
