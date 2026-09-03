import { initializeApp, type FirebaseApp } from "firebase/app";
import {
  getMessaging,
  isSupported,
  onMessage,
  onRegistered,
  onUnregistered,
  register,
  unregister,
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

/** The service worker carries no config of its own — it reads these off its
 * own registration URL. Keeping the values here rather than in that file is
 * what stops the project identity reaching the repository, and what stops the
 * two copies drifting. */
function serviceWorkerURL(): string {
  const params = new URLSearchParams(firebaseConfig);
  return `/firebase-messaging-sw.js?${params.toString()}`;
}

let lastFID: string | undefined;

/** The most recent Firebase Installation ID `onRegistered` delivered, or
 * undefined if this device has never registered in this page's lifetime.
 *
 * Logout needs it: `unregisterFromPush()` resolving does not guarantee
 * `onUnregistered` has already run, and the DELETE it would trigger has to
 * reach the API while the session is still standing. */
export function currentFID(): string | undefined {
  return lastFID;
}

interface PushRegistrationHandlers {
  onRegistered: (fid: string) => void;
  onUnregistered: (fid: string) => void;
}

/** Attaches both FID lifecycle listeners and returns a single unsubscribe that
 * detaches them together.
 *
 * This must be in place before `registerForPush()` runs: the SDK throws
 * `invalid-on-registered-handler` if `register()` is called with no
 * `onRegistered` handler set, and the FID is only ever delivered through that
 * callback — `register()` itself resolves with nothing. Resolves a no-op when
 * Firebase is unconfigured or push is unsupported, so a caller can always
 * treat the return value as safe to call in a cleanup function. */
export async function startPushRegistration(
  handlers: PushRegistrationHandlers,
): Promise<() => void> {
  const instance = await messaging();
  if (!instance) return () => {};

  const stopRegistered = onRegistered(instance, (fid) => {
    lastFID = fid;
    handlers.onRegistered(fid);
  });
  const stopUnregistered = onUnregistered(instance, (fid) => {
    lastFID = undefined;
    handlers.onUnregistered(fid);
  });

  return () => {
    stopRegistered();
    stopUnregistered();
  };
}

/** Registers this installation with FCM. The FID it produces arrives through
 * `startPushRegistration`'s `onRegistered` handler, not from here. Calling it
 * again for an already-registered installation re-delivers the current FID, so
 * it doubles as the silent re-registration a returning visit needs. No-op when
 * Firebase is not configured. */
export async function registerForPush(): Promise<void> {
  const instance = await messaging();
  if (!instance) return;
  const registration =
    await navigator.serviceWorker.register(serviceWorkerURL());
  await register(instance, {
    vapidKey: firebaseVapidKey,
    serviceWorkerRegistration: registration,
  });
}

/** Deletes this installation's FCM registration, which fires `onUnregistered`
 * with the FID that went away. No-op when Firebase is not configured. */
export async function unregisterFromPush(): Promise<void> {
  const instance = await messaging();
  if (!instance) return;
  await unregister(instance);
}

/** Asks the browser for permission and nothing more — registering with FCM is
 * a separate step (see `registerForPush`). Only ever called from an explicit
 * user action, never on page load. */
export async function requestPushPermission(): Promise<PushPermission> {
  if (typeof Notification === "undefined") {
    return "unsupported";
  }
  return Notification.requestPermission();
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
