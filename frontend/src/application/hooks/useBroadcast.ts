import { useState } from "react";
import type { ComponentProps } from "react";
import type { BroadcastTarget, WaAccount } from "@/domain/entities";
// Type-only, deliberately: hooks/index.ts re-exports this module, so a value
// import would pull the modal and its antd tree into every page that imports
// from @/application/hooks. typeof still works in the type position.
import type { BroadcastModal } from "@/presentation/components/cs/BroadcastModal";
import {
  useWaChannels,
  useRefreshWaChannels,
  useBroadcasts,
  useSendBroadcast,
  useSendBroadcastMedia,
} from "./useWaChannels";

/** Typed off the modal's own props rather than redeclared: a prop this hook
 * stops supplying, or the modal stops accepting, is then a compile error
 * instead of something only noticed at runtime. */
type ModalProps = ComponentProps<typeof BroadcastModal>;

interface UseBroadcastResult extends ModalProps {
  /** Opens the modal — what InboxHeaderActions' button calls. */
  onOpen: () => void;
}

/**
 * All the state and wiring behind the "Pengumuman" modal: which channel is
 * picked, the channel and broadcast history, and sending to either or both
 * destinations.
 *
 * Returns exactly the modal's props (plus the opener CsInboxPage's header
 * button needs), so the page spreads this straight onto the component rather
 * than repeating every field.
 */
export function useBroadcast(
  accounts: WaAccount[],
  senderNames: Record<string, string>,
): UseBroadcastResult {
  const [open, setOpen] = useState(false);
  const [selectedChannelId, setSelectedChannelId] = useState<string>();

  const channelsQuery = useWaChannels();
  const broadcastsQuery = useBroadcasts();
  const refreshChannels = useRefreshWaChannels();
  const sendBroadcast = useSendBroadcast();
  const sendBroadcastMedia = useSendBroadcastMedia();

  const channels = channelsQuery.data ?? [];

  // mutateAsync rather than mutate, so the composer learns whether the
  // announcement was queued before it clears what was typed. The rejection is
  // caught and turned into a false: the mutation hook has already shown the
  // reason, and an uncaught rejection here would only add a console error on
  // top of it.
  const handleSend = async (
    body: string,
    targets: BroadcastTarget[],
    file?: File,
  ): Promise<boolean> => {
    if (targets.length === 0) return false;
    try {
      if (file) {
        await sendBroadcastMedia.mutateAsync({ file, caption: body, targets });
      } else {
        await sendBroadcast.mutateAsync({ body, targets });
      }
      return true;
    } catch {
      return false;
    }
  };

  return {
    open,
    channels,
    statusAccountIds: accounts
      .filter((account) => account.status === "connected")
      .map((account) => account.id),
    accountLabels: Object.fromEntries(
      accounts.map((account) => [account.id, account.label]),
    ),
    channelNames: Object.fromEntries(
      channels.map((channel) => [channel.jid, channel.name]),
    ),
    senderNames,
    posts: broadcastsQuery.data ?? [],
    loadingPosts: broadcastsQuery.isLoading,
    refreshing: refreshChannels.isPending,
    sending: sendBroadcast.isPending || sendBroadcastMedia.isPending,
    selectedChannelId,
    onSelectChannel: setSelectedChannelId,
    onRefresh: () => refreshChannels.mutate(),
    onSend: handleSend,
    onClose: () => setOpen(false),
    onOpen: () => setOpen(true),
  };
}
