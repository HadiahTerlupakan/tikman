import { useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import type { WaAccountStatus } from "@/domain/entities";

type CsEvent = {
  type: string;
  conversation_id?: string;
  wa_account_id?: string;
  account_status?: WaAccountStatus;
  pairing_code?: string;
  typing?: boolean;
};

/** How long a "sedang mengetik" line stays up on its own. WhatsApp repeats
 * composing every few seconds while a customer writes and sends paused when
 * they stop — but a paused that never arrives, from a phone that lost signal
 * mid-word, would otherwise leave the line up for the rest of the session. */
const TYPING_TTL_MS = 10_000;

/** How often expired typing lines are swept. Only runs while somebody is
 * actually typing. */
const TYPING_SWEEP_MS = 1_000;

/** How long to wait before reopening a stream the browser gave up on, and the
 * ceiling that wait widens to. Widening rather than fixed: a session that has
 * expired answers 401 to every attempt, and hammering it five seconds apart
 * for the rest of the day buys nothing. */
const REOPEN_DELAY_MS = 3_000;
const MAX_REOPEN_DELAY_MS = 30_000;

/** What this stream has observed live about one number. Undefined until an
 * account_status event for that number arrives on this connection — the caller
 * falls back to the account list for the state at page load. */
export interface WaLiveStatus {
  waStatus?: WaAccountStatus;
  pairingCode?: string;
}

/** Live state per number, keyed by account id. It is a map rather than one
 * value because the team answers from several numbers: a single value would
 * show whichever number changed last as the state of all of them. */
export type WaStreamStatus = Record<string, WaLiveStatus>;

/** Which threads have a customer writing in them right now, keyed by
 * conversation id. */
export type CsTypingStatus = Record<string, boolean>;

/** What the inbox reads back from the stream. */
export interface CsStreamState {
  accounts: WaStreamStatus;
  typing: CsTypingStatus;
}

/**
 * Keeps the inbox current while the page is open.
 *
 * `presence` marks this agent online for as long as the connection lasts, so
 * round-robin only ever hands work to somebody who actually has the inbox in
 * front of them. The app shell runs this hook on every page, so only the inbox
 * route passes it — it is in the effect's dependencies so navigating in and out
 * of the inbox reopens the connection with the right claim.
 *
 * Events carry no data of their own, with one exception: account_status also
 * carries the WhatsApp connection state, and while pairing, the eight-character
 * code an admin types into WhatsApp. Everything else is a nudge to refetch, so
 * a connection that dropped and came back cannot leave the inbox showing a
 * stale thread — the refetch closes whatever gap the outage opened.
 */
export function useCsStream(enabled = true, presence = false): CsStreamState {
  const queryClient = useQueryClient();
  const [waStatus, setWaStatus] = useState<WaStreamStatus>({});
  // Deadlines rather than flags: a customer who is still writing keeps pushing
  // theirs forward, and one whose phone went quiet has theirs pass.
  const [typingUntil, setTypingUntil] = useState<Record<string, number>>({});

  useEffect(() => {
    if (!enabled) return;
    const url = `${env.apiUrl}${API_ENDPOINTS.CS_STREAM}${presence ? "?presence=1" : ""}`;
    let source: EventSource | undefined;
    let reopen: ReturnType<typeof setTimeout> | undefined;
    let delay = REOPEN_DELAY_MS;
    let abandoned = false;

    const onEvent = (event: MessageEvent) => {
      let payload: CsEvent;
      try {
        payload = JSON.parse(event.data);
      } catch {
        return;
      }

      if (payload.type === "typing") {
        // Deliberately before the refetches below: typing changes nothing
        // stored, and a customer holding down a key would otherwise reload the
        // inbox and the whole thread every few seconds.
        applyTyping(setTypingUntil, payload);
        return;
      }

      queryClient.invalidateQueries({ queryKey: ["cs", "conversations"] });
      if (payload.conversation_id) {
        queryClient.invalidateQueries({
          queryKey: ["cs", "messages", payload.conversation_id],
        });
      }
      if (payload.type === "account_status" && payload.wa_account_id) {
        setWaStatus((current) => ({
          ...current,
          [payload.wa_account_id as string]: {
            waStatus: payload.account_status,
            pairingCode: payload.pairing_code,
          },
        }));
      }
    };

    // EventSource reconnects by itself only when the connection drops at the
    // network level. A reconnect that is *answered* with something it will not
    // accept — a 502 from the proxy while the API restarts, a 401 once the
    // session expires — closes it for good, silently: no error the CS can see,
    // just an inbox that stops updating until somebody reloads the page. One
    // deploy used to be enough to do that to every open tab.
    const open = () => {
      const live = new EventSource(url, { withCredentials: true });
      source = live;
      live.addEventListener("cs", onEvent);
      live.onopen = () => {
        delay = REOPEN_DELAY_MS;
      };
      live.onerror = () => {
        // Anything short of CLOSED is EventSource retrying on its own, which
        // is the case this must not interfere with.
        if (live.readyState !== EventSource.CLOSED || abandoned) return;
        live.close();
        reopen = setTimeout(open, delay);
        delay = Math.min(delay * 2, MAX_REOPEN_DELAY_MS);
      };
    };
    open();

    // Closing drops every listener with the connection itself, so there is
    // nothing left to unregister separately.
    return () => {
      abandoned = true;
      clearTimeout(reopen);
      source?.close();
    };
  }, [queryClient, enabled, presence]);

  const typingCount = Object.keys(typingUntil).length;
  useEffect(() => {
    if (typingCount === 0) return;
    const sweep = setInterval(() => {
      setTypingUntil((current) => {
        const now = Date.now();
        const next = Object.fromEntries(
          Object.entries(current).filter(([, until]) => until > now),
        );
        return Object.keys(next).length === Object.keys(current).length
          ? current
          : next;
      });
    }, TYPING_SWEEP_MS);
    return () => clearInterval(sweep);
  }, [typingCount]);

  const typing = useMemo(
    () => Object.fromEntries(Object.keys(typingUntil).map((id) => [id, true])),
    [typingUntil],
  );

  return { accounts: waStatus, typing };
}

/** applyTyping records that one thread started or stopped having a customer
 * write in it. Its own function so the event handler stays readable. */
function applyTyping(
  setTypingUntil: React.Dispatch<React.SetStateAction<Record<string, number>>>,
  payload: CsEvent,
) {
  const id = payload.conversation_id;
  if (!id) return;
  setTypingUntil((current) => {
    if (!payload.typing) {
      if (!(id in current)) return current;
      const next = { ...current };
      delete next[id];
      return next;
    }
    return { ...current, [id]: Date.now() + TYPING_TTL_MS };
  });
}
