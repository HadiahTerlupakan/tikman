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

/** RTDB's own view of the socket. Synthetic and local: it needs no rules and is
 * readable before sign-in. */
const CONNECTED_PATH = ".info/connected";

export const isPresenceConfigured =
  !!firebaseConfig.apiKey && !!firebaseConfig.databaseURL;

/** The narrow shape the claim needs, so its ordering can be tested without the
 * Firebase SDK. */
export interface PresenceRef {
  onDisconnect: () => { remove: () => Promise<unknown> };
  set: (value: unknown) => Promise<unknown>;
  remove: () => Promise<unknown>;
}

/**
 * Arms the disconnect handler, then writes the value.
 *
 * The order is the point. Written first, a socket dying in the gap would leave
 * a node nothing ever removes, and the agent would show as present until the
 * mirror's next pass — the exact failure this design exists to remove.
 */
export async function writePresence(node: PresenceRef): Promise<void> {
  await node.onDisconnect().remove();
  await node.set({ at: serverTimestamp() });
}

/**
 * Holds the node claimed for as long as the browser is connected, and returns
 * the function that gives it up.
 *
 * Claiming once on mount is not enough. When the socket drops the server runs
 * the `onDisconnect` and deletes the node; the SDK then replays listeners and
 * un-acknowledged writes, but not a `set` that already completed, and an
 * `onDisconnect` armed while connected is not queued for replay either. So a
 * single wifi flap would end the shift silently. Re-claiming on every
 * transition to connected is what makes a blip self-heal, the way the SSE
 * heartbeat this replaced used to.
 */
export function claimWhileConnected(
  node: PresenceRef,
  watchConnected: (onChange: (connected: boolean) => void) => () => void,
  onFailed: (error: unknown) => void,
): () => Promise<void> {
  let released = false;

  const stop = watchConnected((connected) => {
    if (!connected || released) return;
    void writePresence(node)
      // Unmounting during the write would otherwise leave the node behind:
      // release() already ran its remove() before this landed.
      .then(async () => {
        if (released) await node.remove();
      })
      .catch(onFailed);
  });

  return async () => {
    released = true;
    stop();
    await node.remove();
  };
}

/**
 * Runs `attach` only once `signIn` has resolved.
 *
 * The gate is not politeness. `/cs-presence` is readable only to `auth != null`,
 * and a listen sent before sign-in is not merely refused: the SDK removes the
 * registration and never re-sends it when the token arrives, so the panel shows
 * nobody for the whole visit with nothing logged anywhere.
 */
export async function attachAfterSignIn(
  signIn: () => Promise<unknown>,
  attach: () => () => void,
): Promise<() => void> {
  await signIn();
  return attach();
}

function app(): FirebaseApp {
  return getApps()[0] ?? initializeApp(firebaseConfig);
}

let signedIn: Promise<string> | undefined;

/** Signs this browser in as the API session's own user, once per page load, and
 * resolves to the uid the presence node is named for. Shared by the claim and
 * the panel's listener so one visit mints one token instead of racing two. */
function ensureSignedIn(): Promise<string> {
  signedIn ??= mintIdentity().catch((error: unknown) => {
    // A failed mint must not poison the rest of the visit: a reconnect or a
    // remount gets to try again.
    signedIn = undefined;
    throw error;
  });
  return signedIn;
}

async function mintIdentity(): Promise<string> {
  const auth = getAuth(app());
  // `auth.authStateReady()` is deliberately not the gate here: it settles the
  // initial state and resolves just as happily with a null user, so it proves
  // nothing about `auth != null`. Nor is a session persisted from an earlier
  // visit reused — it belongs to whoever logged in then, and writing presence
  // under that uid would put this agent in somebody else's seat.
  const token = await csRepository.getFirebaseToken();
  const credential = await signInWithCustomToken(auth, token);
  return credential.user.uid;
}

/** Claims presence for this browser and keeps it claimed across reconnects.
 * Resolves to a no-op when Firebase is not configured, so callers need no
 * branch. */
export async function claimPresence(
  onFailed: (error: unknown) => void,
): Promise<() => Promise<void>> {
  if (!isPresenceConfigured) return async () => {};

  const uid = await ensureSignedIn();
  const node = ref(getDatabase(app()), `${PRESENCE_PATH}/${uid}`);

  return claimWhileConnected(
    {
      onDisconnect: () => onDisconnect(node),
      set: (value) => set(node, value),
      remove: () => remove(node),
    },
    watchConnection,
    onFailed,
  );
}

/** Subscribes to the whole presence set once signed in. Resolves to the
 * unsubscribe function. */
export async function watchPresence(
  onChange: (ids: string[]) => void,
): Promise<() => void> {
  if (!isPresenceConfigured) {
    onChange([]);
    return () => {};
  }
  return attachAfterSignIn(ensureSignedIn, () => {
    const node = ref(getDatabase(app()), PRESENCE_PATH);
    return onValue(
      node,
      (snapshot) => {
        const value = (snapshot.val() ?? {}) as Record<string, unknown>;
        onChange(Object.keys(value));
      },
      (error) => reportCancelled(PRESENCE_PATH, error),
    );
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
  const node = ref(getDatabase(app()), CONNECTED_PATH);
  return onValue(
    node,
    (snapshot) => onChange(snapshot.val() === true),
    (error) => reportCancelled(CONNECTED_PATH, error),
  );
}

/** A cancelled listen is the one Firebase failure with no symptom of its own:
 * the SDK drops the registration and never re-sends it, so the panel just shows
 * nobody. The console is where a developer looks; what the agent sees is the
 * status useOnlineAgents reports. */
function reportCancelled(path: string, error: Error): void {
  console.error(`Firebase presence listener on ${path} was cancelled`, error);
}
