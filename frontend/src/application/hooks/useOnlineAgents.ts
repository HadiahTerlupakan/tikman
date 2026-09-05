import { useEffect, useState } from "react";
import {
  watchConnection,
  watchPresence,
} from "@/infrastructure/firebase/presence";

/**
 * The agents holding the inbox open, straight off the Realtime Database, plus
 * whether this browser is actually still hearing about them.
 *
 * Not React Query: there is nothing to fetch and nothing to invalidate. The
 * subscription pushes, and a query cache in front of it would only add a copy
 * that can be stale.
 */
export function useOnlineAgents(): { data: string[]; connected: boolean } {
  const [ids, setIds] = useState<string[]>([]);
  const [connected, setConnected] = useState(false);

  useEffect(() => {
    let stop: (() => void) | undefined;
    let cancelled = false;

    // The subscription now waits for sign-in, so it can arrive after this
    // component is gone; dropped, the listener would outlive the page.
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

  useEffect(() => watchConnection(setConnected), []);

  return { data: ids, connected };
}
