import { useState } from "react";
import {
  useNavigate,
  useOutletContext,
  useSearchParams,
} from "react-router-dom";
import { Grid, message } from "antd";
import { useAuthStore } from "@/application/stores";
import {
  useCreateWaGroup,
  useRefreshWaGroups,
  useSendWaGroupMedia,
  useSendWaGroupMessage,
  useWaAccounts,
  useWaGroupAction,
  useWaGroupActionTracker,
  useWaGroupDetail,
  useWaGroupHistory,
  useWaGroups,
} from "@/application/hooks";
import { useDebouncedValue } from "@/application/hooks/useDebouncedValue";
import { UserRole } from "@/domain/entities";
import type {
  WaGroup,
  WaGroupAction,
  WaGroupActionRequest,
  WaGroupMessage,
} from "@/domain/entities";
import { describeOutcome } from "@/presentation/components/cs/groups/groupActionOutcome";
import type { AppLayoutContext } from "@/presentation/components/layout/AppLayout";
import { inboxLayout, type InboxPane } from "./inboxLayout";
import { useGroupMessageActions } from "./useGroupMessageActions";

const SEARCH_DEBOUNCE_MS = 300;

/** The selected group and number filter ride the URL, as the inbox's thread
 * does: a link to a group then opens it, and on a phone Back walks the panes. */
function useGroupSelection() {
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const screens = Grid.useBreakpoint();
  const selectedId = searchParams.get("group") ?? undefined;
  const waAccountId = searchParams.get("account") ?? undefined;

  const patchParams = (
    changes: Record<string, string | undefined>,
    push = false,
  ) => {
    const next = new URLSearchParams(searchParams);
    for (const [key, value] of Object.entries(changes)) {
      if (value === undefined) next.delete(key);
      else next.set(key, value);
    }
    setSearchParams(next, { replace: !push });
  };

  // The inbox's layout rule carries over unchanged; its third pane is the
  // customer there and the group's info here.
  const panel = searchParams.get("panel") === "info" ? "customer" : null;
  const layout = inboxLayout(screens, selectedId, panel);
  const showsPane = (pane: InboxPane) => layout.columns || layout.pane === pane;

  return {
    navigate,
    screens,
    selectedId,
    waAccountId,
    patchParams,
    layout,
    showsPane,
  };
}

function useGroupSending(group: WaGroup | undefined) {
  const [replyTo, setReplyTo] = useState<WaGroupMessage>();
  const sendMessage = useSendWaGroupMessage();
  const sendMedia = useSendWaGroupMedia();

  // Resolves to false rather than rejecting: the mutation has already shown
  // why, and the composer only needs to know whether to keep the draft.
  const run = async (send: (groupId: string) => Promise<unknown>) => {
    if (!group) return false;
    try {
      await send(group.id);
      setReplyTo(undefined);
      return true;
    } catch {
      return false;
    }
  };

  const handleSend = (body: string, mentions: string[] = []) =>
    run((groupId) =>
      sendMessage.mutateAsync({
        groupId,
        body,
        replyToId: replyTo?.id,
        mentions,
      }),
    );
  const handleAttach = (file: File, caption: string, mentions: string[] = []) =>
    run((groupId) =>
      sendMedia.mutateAsync({
        groupId,
        file,
        caption,
        replyToId: replyTo?.id,
        mentions,
      }),
    );

  return {
    replyTo,
    setReplyTo,
    sendMessage,
    sendMedia,
    handleSend,
    handleAttach,
  };
}

function useGroupManagement(onCreated: (groupId: string) => void) {
  const [inviteLink, setInviteLink] = useState<string>();
  const [createOpen, setCreateOpen] = useState(false);
  const action = useWaGroupAction();
  const create = useCreateWaGroup();

  const track = useWaGroupActionTracker((settled: WaGroupAction) => {
    const outcome = describeOutcome(settled);
    message[outcome.level](outcome.text);
    if (outcome.inviteLink) setInviteLink(outcome.inviteLink);
    if (settled.action === "create" && settled.groupId) {
      onCreated(settled.groupId);
    }
  });

  const queued = (queuedAction: WaGroupAction) => {
    message.info("Permintaan diteruskan ke WhatsApp");
    track(queuedAction);
  };

  const runAction = (groupId: string, request: WaGroupActionRequest) =>
    action.mutate({ groupId, request }, { onSuccess: queued });

  const createGroup = (vars: Parameters<typeof create.mutate>[0]) =>
    create.mutate(vars, {
      onSuccess: (queuedAction) => {
        setCreateOpen(false);
        queued(queuedAction);
      },
    });

  return {
    inviteLink,
    closeInviteLink: () => setInviteLink(undefined),
    createOpen,
    setCreateOpen,
    creating: create.isPending,
    createGroup,
    acting: action.isPending,
    runAction,
  };
}

/**
 * Everything the groups screen runs on, apart from its markup: the URL it
 * reads the group from, the queries behind the three panes, sending, and the
 * admin's management actions.
 */
export function useCsGroups() {
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === UserRole.ADMIN;
  const selection = useGroupSelection();
  const { selectedId, waAccountId, patchParams, layout } = selection;

  const [search, setSearch] = useState("");
  const debouncedSearch = useDebouncedValue(search.trim(), SEARCH_DEBOUNCE_MS);
  const groupsQuery = useWaGroups({
    waAccountId,
    search: debouncedSearch || undefined,
  });
  const detailQuery = useWaGroupDetail(selectedId);
  const history = useWaGroupHistory(selectedId);
  const accounts = useWaAccounts().data ?? [];
  const refresh = useRefreshWaGroups();

  const groups = groupsQuery.data ?? [];
  // The detail covers a group the current search or filter hides, such as one
  // opened from a link.
  const selected =
    groups.find((g) => g.id === selectedId) ?? detailQuery.data?.group;
  const sending = useGroupSending(selected);

  const handleSelect = (id: string) => {
    patchParams({ group: id, panel: undefined }, !layout.columns);
    sending.setReplyTo(undefined);
  };
  const management = useGroupManagement(handleSelect);
  const messageActions = useGroupMessageActions(selected, sending.setReplyTo);
  // Absent outside the app shell, which is where the stream runs.
  const groupTyping =
    useOutletContext<AppLayoutContext | undefined>()?.csGroupTyping ?? {};

  return {
    isAdmin,
    currentUserId: currentUser?.id ?? "",
    ...selection,
    search,
    setSearch,
    accounts,
    groups,
    selected,
    detailQuery,
    history,
    refresh,
    handleSelect,
    setAccount: (id?: string) => patchParams({ account: id }),
    ...sending,
    ...management,
    messageActions,
    groupTyping,
  };
}
