import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { message } from "antd";
import {
  useCreateWaGroupPoll,
  useEditWaGroupMessage,
  useForwardWaGroupMessage,
  usePinWaGroup,
  useReactToWaGroupMessage,
  useReplyPrivatelyToWaGroupMessage,
  useRequestWaGroupHistory,
  useRevokeWaGroupMessage,
  useSetWaGroupTyping,
  useStarWaGroupMessage,
  useVoteWaGroupPoll,
  useWaGroupActionTracker,
} from "@/application/hooks";
import type {
  WaGroup,
  WaGroupAction,
  WaGroupMessage,
  WaGroupPollRequest,
} from "@/domain/entities";
import type { GroupMessageDialogState } from "@/presentation/components/cs/groups/GroupMessageDialogs";
import type { GroupMessageHandlers } from "@/presentation/components/cs/groups/groupMessageActions";

/**
 * Follows what WhatsApp has to carry out for a message (a reaction, an edit,
 * a delete, a vote, a history request) far enough to say when it failed. A
 * success needs no word: the group_message event that follows redraws the
 * thread. Anyone may read back an action they asked for themselves.
 */
function useQueuedMessageAction() {
  const track = useWaGroupActionTracker((settled: WaGroupAction) => {
    if (settled.status === "failed") {
      message.error(settled.failReason || "WhatsApp menolak permintaan ini");
    }
  });
  return (action: WaGroupAction) => track(action);
}

/** Everything that acts on one message, or on the group as a chat rather
 * than as something an admin manages. */
export function useGroupMessageActions(
  group: WaGroup | undefined,
  onReply: (message: WaGroupMessage) => void,
) {
  const navigate = useNavigate();
  const [dialog, setDialog] = useState<GroupMessageDialogState>();
  const queued = useQueuedMessageAction();
  const react = useReactToWaGroupMessage();
  const revoke = useRevokeWaGroupMessage();
  const vote = useVoteWaGroupPoll();
  const star = useStarWaGroupMessage();
  const replyPrivate = useReplyPrivatelyToWaGroupMessage();
  const ref = (m: WaGroupMessage) => ({ groupId: m.groupId, messageId: m.id });

  const handlers: GroupMessageHandlers = {
    onReply,
    onReact: (m, emoji) =>
      react.mutate({ ...ref(m), emoji }, { onSuccess: queued }),
    onForward: (m) => setDialog({ kind: "forward", message: m }),
    onStar: (m) => star.mutate({ ...ref(m), starred: !m.starred }),
    onInfo: (m) => setDialog({ kind: "info", message: m }),
    onEdit: (m) => setDialog({ kind: "edit", message: m }),
    onRevoke: (m) => revoke.mutate(ref(m), { onSuccess: queued }),
    onReplyPrivate: (m) =>
      replyPrivate.mutate(ref(m), {
        onSuccess: (conversationId) =>
          navigate(`/cs?conversation=${conversationId}`),
      }),
    onVote: (m, options) =>
      vote.mutate({ ...ref(m), options }, { onSuccess: queued }),
  };

  return {
    handlers,
    dialog,
    closeDialog: () => setDialog(undefined),
    ...useMessageDialogs(queued, () => setDialog(undefined)),
    ...useGroupChatActions(group, queued),
  };
}

function useMessageDialogs(
  queued: (action: WaGroupAction) => void,
  close: () => void,
) {
  const edit = useEditWaGroupMessage();
  const forward = useForwardWaGroupMessage();
  return {
    submitEdit: (m: WaGroupMessage, body: string) =>
      edit.mutate(
        { groupId: m.groupId, messageId: m.id, body },
        {
          onSuccess: (action) => {
            close();
            queued(action);
          },
        },
      ),
    forwarding: forward.isPending,
    submitForward: (m: WaGroupMessage, groupIds: string[]) =>
      forward.mutate(
        { groupId: m.groupId, messageId: m.id, groupIds },
        {
          onSuccess: () => {
            close();
            message.success(`Diteruskan ke ${groupIds.length} grup`);
          },
        },
      ),
  };
}

function useGroupChatActions(
  group: WaGroup | undefined,
  queued: (action: WaGroupAction) => void,
) {
  const pin = usePinWaGroup();
  const history = useRequestWaGroupHistory();
  const createPoll = useCreateWaGroupPoll();
  const setTyping = useSetWaGroupTyping();

  const submitPoll = async (poll: WaGroupPollRequest) => {
    if (!group) return false;
    try {
      await createPoll.mutateAsync({ groupId: group.id, poll });
      return true;
    } catch {
      // The mutation has already said why.
      return false;
    }
  };

  return {
    togglePin: (g: WaGroup) => pin.mutate({ groupId: g.id, pinned: !g.pinned }),
    requestingHistory: history.isPending,
    requestHistory: () =>
      group &&
      history.mutate(group.id, {
        onSuccess: (action) => {
          message.info(
            "Permintaan dikirim ke HP. Pesan lama muncul setelah HP mengirimkannya.",
          );
          queued(action);
        },
      }),
    creatingPoll: createPoll.isPending,
    submitPoll,
    setTyping,
  };
}
