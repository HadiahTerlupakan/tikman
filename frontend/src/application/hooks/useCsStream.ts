import { useEffect, useState } from "react";
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
};

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
export function useCsStream(enabled = true, presence = false): WaStreamStatus {
  const queryClient = useQueryClient();
  const [waStatus, setWaStatus] = useState<WaStreamStatus>({});

  useEffect(() => {
    if (!enabled) return;
    const url = `${env.apiUrl}${API_ENDPOINTS.CS_STREAM}${presence ? "?presence=1" : ""}`;
    const source = new EventSource(url, {
      withCredentials: true,
    });

    const onEvent = (event: MessageEvent) => {
      let payload: CsEvent;
      try {
        payload = JSON.parse(event.data);
      } catch {
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

    source.addEventListener("cs", onEvent);
    // Closing drops every listener with the connection itself, so there is
    // nothing left to unregister separately.
    return () => source.close();
  }, [queryClient, enabled, presence]);

  return waStatus;
}
