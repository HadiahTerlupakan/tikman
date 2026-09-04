import { useState } from "react";
import type { ComponentProps } from "react";
import type { WaAccount } from "@/domain/entities";
// Type-only, deliberately: hooks/index.ts re-exports this module, so a value
// import would pull the modal and its antd tree into every page that imports
// from @/application/hooks. typeof still works in the type position.
import type { ChannelBroadcastModal } from "@/presentation/components/cs/ChannelBroadcastModal";
import {
  useWaChannels,
  useRefreshWaChannels,
  useChannelPosts,
  useSendChannelPost,
  useSendChannelPostMedia,
} from "./useWaChannels";

/** Typed off the modal's own props rather than redeclared: a prop this hook
 * stops supplying, or the modal stops accepting, is then a compile error
 * instead of something only noticed at runtime. */
type ModalProps = ComponentProps<typeof ChannelBroadcastModal>;

interface UseChannelBroadcastResult extends ModalProps {
  /** Opens the modal — what InboxHeaderActions' button calls. */
  onOpen: () => void;
}

/**
 * All the state and wiring behind the "Pembaruan Saluran" modal: which
 * channel is picked, the channel list and its history, and sending an update.
 *
 * Returns exactly the modal's props (plus the opener CsInboxPage's header
 * button needs), so the page spreads this straight onto the component rather
 * than repeating every field.
 */
export function useChannelBroadcast(
  accounts: WaAccount[],
  senderNames: Record<string, string>,
): UseChannelBroadcastResult {
  const [open, setOpen] = useState(false);
  const [selectedChannelId, setSelectedChannelId] = useState<string>();

  const channelsQuery = useWaChannels();
  const channelPostsQuery = useChannelPosts(selectedChannelId);
  const refreshChannels = useRefreshWaChannels();
  const sendChannelPost = useSendChannelPost();
  const sendChannelPostMedia = useSendChannelPostMedia();

  // mutateAsync rather than mutate, so the composer learns whether the update
  // was queued before it clears what was typed. The rejection is caught and
  // turned into a false: the mutation hook has already shown the reason, and
  // an uncaught rejection here would only add a console error on top of it.
  const handleSend = async (body: string, file?: File): Promise<boolean> => {
    if (!selectedChannelId) return false;
    try {
      if (file) {
        await sendChannelPostMedia.mutateAsync({
          channelId: selectedChannelId,
          file,
          caption: body,
        });
      } else {
        await sendChannelPost.mutateAsync({
          channelId: selectedChannelId,
          body,
        });
      }
      return true;
    } catch {
      return false;
    }
  };

  return {
    open,
    channels: channelsQuery.data ?? [],
    accountLabels: Object.fromEntries(
      accounts.map((account) => [account.id, account.label]),
    ),
    senderNames,
    posts: channelPostsQuery.data ?? [],
    loadingPosts: channelPostsQuery.isLoading,
    refreshing: refreshChannels.isPending,
    sending: sendChannelPost.isPending || sendChannelPostMedia.isPending,
    selectedChannelId,
    onSelectChannel: setSelectedChannelId,
    onRefresh: () => refreshChannels.mutate(),
    onSend: handleSend,
    onClose: () => setOpen(false),
    onOpen: () => setOpen(true),
  };
}
