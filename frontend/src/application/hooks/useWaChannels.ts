import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { CsRepository } from "@/infrastructure/repositories";
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

/** One channel's broadcast history. This is where a sender learns whether
 * their update actually went: sending only queues it. */
export function useChannelPosts(channelId?: string) {
  return useQuery({
    queryKey: ["cs", "channel-posts", channelId],
    queryFn: () => csRepository.getChannelPosts(channelId as string),
    enabled: !!channelId,
  });
}

export function useSendChannelPost() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { channelId: string; body: string }) =>
      csRepository.sendChannelPost(vars.channelId, vars.body),
    onSuccess: (_post, vars) => {
      queryClient.invalidateQueries({
        queryKey: ["cs", "channel-posts", vars.channelId],
      });
    },
    onError: reportCsMutationError,
  });
}

export function useSendChannelPostMedia() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (vars: { channelId: string; file: File; caption?: string }) =>
      csRepository.sendChannelPostMedia(
        vars.channelId,
        vars.file,
        vars.caption,
      ),
    onSuccess: (_post, vars) => {
      queryClient.invalidateQueries({
        queryKey: ["cs", "channel-posts", vars.channelId],
      });
    },
    onError: reportCsMutationError,
  });
}
