import { initializeApp, type FirebaseApp } from "firebase/app";
import {
  getMessaging,
  getToken,
  isSupported,
  onMessage,
  type Messaging,
} from "firebase/messaging";
import {
  firebaseConfig,
  firebaseVapidKey,
  isFirebaseConfigured,
} from "@/shared/config/firebase";

let app: FirebaseApp | undefined;
let messagingInstance: Messaging | undefined;

/** Lazily creates the Firebase app and Messaging instance on first real use,
 * never at module load — importing this file must have no effect on a
 * checkout with no Firebase project configured. */
async function messaging(): Promise<Messaging | undefined> {
  if (!isFirebaseConfigured) return undefined;
  if (!(await isSupported())) return undefined;
  if (!messagingInstance) {
    app ??= initializeApp(firebaseConfig);
    messagingInstance = getMessaging(app);
  }
  return messagingInstance;
}

export type PushPermission = "default" | "granted" | "denied" | "unsupported";

function currentPermission(): PushPermission {
  if (typeof Notification === "undefined") return "unsupported";
  return Notification.permission;
}

/** The service worker carries no config of its own — it reads these off its
 * own registration URL. Keeping the values here rather than in that file is
 * what stops the project identity reaching the repository, and what stops the
 * two copies drifting. */
function serviceWorkerURL(): string {
  const params = new URLSearchParams(firebaseConfig);
  return `/firebase-messaging-sw.js?${params.toString()}`;
}

async function registerAndGetToken(): Promise<string | undefined> {
  const instance = await messaging();
  if (!instance) return undefined;
  const registration =
    await navigator.serviceWorker.register(serviceWorkerURL());
  return getToken(instance, {
    vapidKey: firebaseVapidKey,
    serviceWorkerRegistration: registration,
  });
}

/** Asks the browser for permission, registers the service worker, and
 * returns the device token to hand the backend. Only ever called from an
 * explicit user action — never on page load. */
export async function requestPushPermission(): Promise<{
  permission: PushPermission;
  token?: string;
}> {
  if (typeof Notification === "undefined") {
    return { permission: "unsupported" };
  }
  const permission = await Notification.requestPermission();
  if (permission !== "granted") {
    return { permission };
  }
  const token = await registerAndGetToken();
  return { permission, token };
}

/** Re-fetches the device token without prompting, for a session where
 * permission was already granted on a previous visit. Resolves undefined
 * whenever permission is not already granted or Firebase is not configured. */
export async function refreshTokenIfGranted(): Promise<string | undefined> {
  if (currentPermission() !== "granted") return undefined;
  return registerAndGetToken();
}

/** Shows a notification for a push that arrives while a tab is focused — FCM
 * only runs the service worker's background handler otherwise.
 *
 * The `Notification` constructor is not an option here: Chrome for Android
 * throws `TypeError: Illegal constructor` for it, which is exactly the device a
 * CS answers from. Only the service worker registration can display one there. */
export async function showLocalNotification(
  title: string,
  body: string,
): Promise<void> {
  const registration = await navigator.serviceWorker.ready;
  await registration.showNotification(title, { body });
}

/** Listens for pushes that arrive while a tab is focused. The payload is
 * data-only (see backend internal/push/client.go), so title and body come from
 * `data`, not from a notification block. Resolves a no-op unsubscribe when
 * Firebase is not configured, so a caller can always treat the return value as
 * safe to call in a cleanup function. */
export async function listenForForegroundMessages(
  onIncoming: (title: string, body: string) => void,
): Promise<() => void> {
  const instance = await messaging();
  if (!instance) return () => {};
  return onMessage(instance, (payload) => {
    onIncoming(payload.data?.title ?? "", payload.data?.body ?? "");
  });
}
