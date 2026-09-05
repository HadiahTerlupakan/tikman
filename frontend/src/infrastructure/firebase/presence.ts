import { initializeApp, getApps, type FirebaseApp } from "firebase/app";
import { getAuth, signInWithCustomToken } from "firebase/auth";
import {
  getDatabase,
  ref,
  serverTimestamp,
  onDisconnect,
  onValue,
  set,
  remove,
} from "firebase/database";
import { firebaseConfig } from "@/shared/config/firebase";
import { CsRepository } from "@/infrastructure/repositories";

const csRepository = new CsRepository();

/** The path the security rules and the Go mirror both name. Changing it here
 * alone silently breaks both. */
const PRESENCE_PATH = "cs-presence";

export const isPresenceConfigured =
  !!firebaseConfig.apiKey && !!firebaseConfig.databaseURL;

/** The narrow shape writePresence needs, so its ordering can be tested
 * without the Firebase SDK. */
export interface PresenceRef {
  onDisconnect: () => { remove: () => Promise<unknown> };
  set: (value: unknown) => Promise<unknown>;
  remove: () => Promise<unknown>;
}

/**
 * Claims a presence node and returns the function that gives it up.
 *
 * The disconnect handler is armed BEFORE the value is written. Written first,
 * a socket dying in the gap would leave a node nothing ever removes, and the
 * agent would show as present until the mirror's next pass — the exact failure
 * this design exists to remove.
 */
export async function writePresence(
  node: PresenceRef,
): Promise<() => Promise<void>> {
  await node.onDisconnect().remove();
  await node.set({ at: serverTimestamp() });
  return async () => {
    await node.remove();
  };
}

function app(): FirebaseApp {
  return getApps()[0] ?? initializeApp(firebaseConfig);
}

/** Signs this browser in as its own user and claims presence for it. Resolves
 * to a no-op when Firebase is not configured, so callers need no branch. */
export async function claimPresence(): Promise<() => Promise<void>> {
  if (!isPresenceConfigured) return async () => {};

  const token = await csRepository.getFirebaseToken();
  const auth = getAuth(app());
  const credential = await signInWithCustomToken(auth, token);

  const node = ref(
    getDatabase(app()),
    `${PRESENCE_PATH}/${credential.user.uid}`,
  );
  return writePresence({
    onDisconnect: () => onDisconnect(node),
    set: (value) => set(node, value),
    remove: () => remove(node),
  });
}

/** Subscribes to the whole presence set. Returns the unsubscribe function. */
export function watchPresence(onChange: (ids: string[]) => void): () => void {
  if (!isPresenceConfigured) {
    onChange([]);
    return () => {};
  }
  const node = ref(getDatabase(app()), PRESENCE_PATH);
  return onValue(node, (snapshot) => {
    const value = (snapshot.val() ?? {}) as Record<string, unknown>;
    onChange(Object.keys(value));
  });
}

/** Subscribes to RTDB's own view of whether this browser is connected.
 * Reports false when Firebase is not configured, which is the truth. */
export function watchConnection(
  onChange: (connected: boolean) => void,
): () => void {
  if (!isPresenceConfigured) {
    onChange(false);
    return () => {};
  }
  const node = ref(getDatabase(app()), ".info/connected");
  return onValue(node, (snapshot) => onChange(snapshot.val() === true));
}
