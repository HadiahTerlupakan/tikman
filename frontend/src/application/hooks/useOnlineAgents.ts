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

  useEffect(() => watchPresence(setIds), []);
  useEffect(() => watchConnection(setConnected), []);

  return { data: ids, connected };
}
