import { useEffect, useState } from "react";
import {
  claimPresence,
  watchConnection,
  watchPresence,
} from "@/infrastructure/firebase/presence";

/**
 * What the inbox has to say out loud about presence.
 *
 * `ok` deliberately covers three different healthy-enough situations — a
 * deployment with no Firebase project at all, a connection still coming up, and
 * a live one — because none of them is something an agent can act on, and a
 * permanent banner is how a real warning stops being read. The other two are
 * the cases that cost something: a list frozen at its last snapshot, and an
 * agent the round-robin cannot reach.
 */
export type PresenceStatus = "ok" | "stale" | "unclaimed";

/**
 * The agents holding the inbox open, straight off the Realtime Database, plus
 * whether this browser is still hearing about them and still in the rotation
 * itself.
 *
 * Claiming lives here rather than in the page because the claim and the panel
 * are one subscription's two ends: they share the sign-in, and the failure of
 * either is what `status` reports. Only the CS Inbox calls this hook, which is
 * what keeps someone reading the OLT map out of the rotation.
 *
 * Not React Query: there is nothing to fetch and nothing to invalidate. The
 * subscription pushes, and a query cache in front of it would only add a copy
 * that can be stale.
 */
export function useOnlineAgents(): { data: string[]; status: PresenceStatus } {
  const [ids, setIds] = useState<string[]>([]);
  const [stale, setStale] = useState(false);
  const [unclaimed, setUnclaimed] = useState(false);

  useEffect(() => {
    let stop: (() => void) | undefined;
    let cancelled = false;

    watchPresence(setIds)
      .then((unsubscribe) => {
        if (cancelled) unsubscribe();
        else stop = unsubscribe;
      })
      .catch((error) => console.error("Could not watch presence", error));

    return () => {
      cancelled = true;
      stop?.();
    };
  }, []);

  useEffect(() => {
    // `.info/connected` reports false until the socket is actually up, and the
    // panel is empty at that point: saying an empty list may be inaccurate is
    // noise on every healthy page load. Only a connection that existed and went
    // away leaves a frozen snapshot worth warning about.
    let connectedOnce = false;
    return watchConnection((link) => {
      if (link === "connected") {
        connectedOnce = true;
        setStale(false);
      } else if (link === "disconnected" && connectedOnce) {
        setStale(true);
      }
    });
  }, []);

  useEffect(() => {
    let release: (() => Promise<void>) | undefined;
    let cancelled = false;

    const failed = (error: unknown) => {
      console.warn("Could not claim presence", error);
      setUnclaimed(true);
    };

    claimPresence({ onClaimed: () => setUnclaimed(false), onFailed: failed })
      .then((r) => {
        if (cancelled) {
          void r();
          return;
        }
        release = r;
      })
      .catch(failed);

    return () => {
      cancelled = true;
      void release?.();
    };
  }, []);

  return {
    data: ids,
    // A claim that is not standing outranks a stale list: the list being a
    // little old costs the agent nothing, being out of the rotation costs them
    // the shift.
    status: unclaimed ? "unclaimed" : stale ? "stale" : "ok",
  };
}
