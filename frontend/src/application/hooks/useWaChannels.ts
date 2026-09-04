import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
import type { BroadcastTarget } from "@/domain/entities";
import { reportCsMutationError } from "./csMutationError";

const csRepository = new CsRepository();

const CHANNELS_KEY = ["cs", "wa-channels"];

/** The channels the team may post to, mirrored from WhatsApp by the wa
 * process. This never touches a WhatsApp connection. */
export function useWaChannels() {
  return useQuery({
    queryKey: CHANNELS_KEY,
    queryFn: () => csRepository.listWaChannels(),
  });
}

/** Asks the wa process to re-read its channel lists. The mirror refreshes
 * hourly by itself; this is for an admin who has just been given a channel and
 * does not want to wait. The rows change a moment after this resolves, so the
 * list is invalidated rather than read from the response. */
export function useRefreshWaChannels() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: () => csRepository.refreshWaChannels(),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: CHANNELS_KEY });
    },
    onError: reportCsMutationError,
  });
}

const BROADCASTS_KEY = ["cs", "broadcasts"];

/** The most recent announcements across every destination. This is where a
 * sender learns whether theirs actually went: sending only queues it. */
export function useBroadcasts() {
  return useQuery({
    queryKey: BROADCASTS_KEY,
    queryFn: () => csRepository.getBroadcasts(),
  });
}

export function useSendBroadcast() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { body: string; targets: BroadcastTarget[] }) =>
      csRepository.sendBroadcast(vars.body, vars.targets),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BROADCASTS_KEY });
    },
    onError: reportCsMutationError,
  });
}

export function useSendBroadcastMedia() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: {
      file: File;
      caption: string;
      targets: BroadcastTarget[];
    }) =>
      csRepository.sendBroadcastMedia(vars.file, vars.caption, vars.targets),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: BROADCASTS_KEY });
    },
    onError: reportCsMutationError,
  });
}
