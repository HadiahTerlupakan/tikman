import { useState } from "react";
import {
  registerForPush,
  requestPushPermission,
  type PushPermission,
} from "@/infrastructure/firebase/messaging";
import { isFirebaseConfigured } from "@/shared/config/firebase";

/** What the browser can do about push, given how this build is configured.
 * With no Firebase project, requestPushPermission spends the browser's one
 * permission prompt and nothing can register — so the control must not be
 * offered at all, which is what "unsupported" makes PushOptInButton do. */
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

/** Drives the CS Inbox "Aktifkan notifikasi" control. */
export function usePushNotifications(): UsePushNotificationsResult {
  const [permission, setPermission] =
    useState<PushPermission>(initialPermission);
  const [requesting, setRequesting] = useState(false);

  const enable = async () => {
    setRequesting(true);
    try {
      const granted = await requestPushPermission();
      setPermission(granted);
      if (granted === "granted") {
        // Registering does not hand back the FID — it arrives asynchronously
        // through onRegistered, which AppLayout listens on for the whole app
        // shell. That listener is the one place that tells the backend about
        // this device, so nothing is POSTed from here.
        await registerForPush();
      }
    } finally {
      setRequesting(false);
    }
  };

  return { permission, requesting, enable };
}
