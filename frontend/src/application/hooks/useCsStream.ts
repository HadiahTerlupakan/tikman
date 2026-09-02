import { useEffect } from "react";
import { useQueryClient } from "@tanstack/react-query";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";

type CsEvent = {
  type: string;
  conversation_id?: string;
};

/**
 * Keeps the inbox current while the page is open, and marks this agent online
 * for as long as the connection lasts — round-robin only ever hands work to
 * somebody who actually has the inbox in front of them.
 *
 * Events carry no data of their own. Each one is a nudge to refetch, so a
 * connection that dropped and came back cannot leave the inbox showing a stale
 * thread: the refetch closes whatever gap the outage opened.
 */
export function useCsStream() {
  const queryClient = useQueryClient();

  useEffect(() => {
    const source = new EventSource(`${env.apiUrl}${API_ENDPOINTS.CS_STREAM}`, {
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
    };

    source.addEventListener("cs", onEvent);
    // Closing drops every listener with the connection itself, so there is
    // nothing left to unregister separately.
    return () => source.close();
  }, [queryClient]);
}
