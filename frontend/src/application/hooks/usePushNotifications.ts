import { useState } from "react";
import {
  requestPushPermission,
  type PushPermission,
} from "@/infrastructure/firebase/messaging";
import { PushRepository } from "@/infrastructure/repositories";
import { isFirebaseConfigured } from "@/shared/config/firebase";

const pushRepository = new PushRepository();

/** What the browser can do about push, given how this build is configured.
 * With no Firebase project, requestPushPermission spends the browser's one
 * permission prompt and returns no token — so the control must not be offered
 * at all, which is what "unsupported" makes PushOptInButton do. */
function initialPermission(): PushPermission {
  if (!isFirebaseConfigured) return "unsupported";
  if (typeof Notification === "undefined") return "unsupported";
  return Notification.permission;
}

interface UsePushNotificationsResult {
  permission: PushPermission;
  requesting: boolean;
  enable: () => Promise<void>;
}

/** Drives the CS Inbox "Aktifkan notifikasi" control. Registering the device
 * on the backend happens here — infrastructure/firebase/messaging.ts only
 * knows about the browser and Firebase side of push. */
export function usePushNotifications(): UsePushNotificationsResult {
  const [permission, setPermission] =
    useState<PushPermission>(initialPermission);
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
