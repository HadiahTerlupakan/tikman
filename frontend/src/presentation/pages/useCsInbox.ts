import { useState } from "react";
import {
  useNavigate,
  useOutletContext,
  useSearchParams,
} from "react-router-dom";
import { Grid } from "antd";
import { useAuthStore } from "@/application/stores";
import {
  useAssignConversation,
  useConnectWaAccount,
  useDisconnectWaAccount,
  useCsConversations,
  useOnlineAgents,
  useCsHistory,
  useCreateWaAccount,
  useCsQuickReplies,
  useClearCsConversation,
  useClearCsInbox,
  useClearWaAccountMessages,
  useDeleteCsMessage,
  useDeleteWaAccount,
  useSendCsMedia,
  useSendCsMessage,
  useSetCsTyping,
  useUsers,
  useWaAccounts,
  usePushNotifications,
  useBroadcast,
} from "@/application/hooks";
import type { PresenceStatus } from "@/application/hooks";
import { UserRole } from "@/domain/entities";
import type { CsMessage, User, WaAccount } from "@/domain/entities";
import {
  filterFor,
  isInboxView,
  type InboxView,
} from "@/presentation/components/cs/InboxFilterBar";
import { inboxLayout, type InboxPane } from "./inboxLayout";
import { colors } from "@/shared/theme/colors";
import type { AppLayoutContext } from "@/presentation/components/layout/AppLayout";

// Only the two statuses an agent can act on say anything. A deployment with no
// Firebase project, and a connection still coming up, are both silent: a banner
// standing permanently under an empty panel is how the real warnings stop being
// read.
const presenceNotice: Partial<
  Record<PresenceStatus, { text: string; color: string }>
> = {
  stale: {
    text: "Terputus — daftar ini mungkin sudah tidak akurat",
    color: colors.textMuted,
  },
  // The one an agent needs most: the list being a little old costs them
  // nothing, not being in the rotation costs them the shift.
  unclaimed: {
    text: "Belum masuk rotasi — muat ulang halaman agar chat baru masuk",
    color: colors.warning,
  },
};

function holderNameMap(users: User[]): Record<string, string> {
  return Object.fromEntries(users.map((u) => [u.id, u.username]));
}

/**
 * Everything the inbox screen runs on: the URL it reads its view and thread
 * from, the queries behind the three columns, and the sends a CS makes. It
 * lives apart from the markup because the screen is mostly markup, and mixing
 * the two made one file nobody could scan.
 */
export function useCsInbox() {
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === UserRole.ADMIN;

  // The three columns are meant to fit the screen exactly, so the height is
  // derived from the shell's own constants rather than written down.
  const screens = Grid.useBreakpoint();

  const [numbersOpen, setNumbersOpen] = useState(false);
  // The number whose pairing panel is open, if any. Pairing is per number now,
  // so the panel has to be told which one rather than assuming the only one.
  const [pairing, setPairing] = useState<WaAccount>();
  const [quickRepliesOpen, setQuickRepliesOpen] = useState(false);
  // The URL owns the view, rather than component state seeded from it. The
  // navbar bell links here with ?view=belum-dibalas, and a CS clicking it while
  // already on this page navigates within the same route: nothing remounts, so
  // a useState initial value would be read once and never again — the click
  // would change the address bar and nothing else. Deriving it means the list
  // always matches the URL, and the view survives a reload or a shared link.
  //
  // "Semua" is the fallback, not "Milik saya": a CS opening the inbox needs to
  // see what nobody has picked up, not only what is already theirs.
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const requestedView = searchParams.get("view");
  const view: InboxView = isInboxView(requestedView) ? requestedView : "semua";

  // The selected thread rides the URL too, for the same reason and one more:
  // the bell's panel links straight to a thread, so a CS clicking a second
  // notification while already here navigates within this route. Held as state
  // it would be set once and ignored afterwards. It also makes a thread a
  // link somebody can send a colleague.
  const selectedId = searchParams.get("conversation") ?? undefined;

  // replace, so a session of switching tabs and threads does not leave Back
  // walking through every one of them.
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

  const setView = (next: InboxView) =>
    patchParams({ view: next === "semua" ? undefined : next });
  const [search, setSearch] = useState("");
  const [replyTo, setReplyTo] = useState<CsMessage>();

  const { csStream: stream, csTyping } = useOutletContext<AppLayoutContext>();
  const setTyping = useSetCsTyping();
  const push = usePushNotifications();
  const conversationsQuery = useCsConversations(filterFor(view, search));
  const historyQuery = useCsHistory(selectedId);
  const usersQuery = useUsers();
  const onlineQuery = useOnlineAgents();
  const quickRepliesQuery = useCsQuickReplies();
  // Reading the account list is open to the whole inbox team, not just an
  // admin — a connection down for hours produces no live SSE event, so this
  // initial fetch is what makes the badge honest for a CS or technician too.
  const accountsQuery = useWaAccounts();
  const accounts = accountsQuery.data ?? [];
  const holderNames = holderNameMap(usersQuery.data ?? []);
  const broadcast = useBroadcast(accounts, holderNames);

  const sendMessage = useSendCsMessage();
  const sendMedia = useSendCsMedia();
  const assignConversation = useAssignConversation();
  const connectAccount = useConnectWaAccount();
  const createAccount = useCreateWaAccount();
  const disconnectAccount = useDisconnectWaAccount();
  const deleteAccount = useDeleteWaAccount();
  const deleteMessage = useDeleteCsMessage();
  const clearConversation = useClearCsConversation();
  const clearAccountMessages = useClearWaAccountMessages();
  const clearInbox = useClearCsInbox();

  const notice = presenceNotice[onlineQuery.status];

  const conversations = conversationsQuery.data ?? [];
  const selected = conversations.find((c) => c.id === selectedId);
  // A live status from this session's own stream is more current than the
  // fetch it started from — once one arrives for a number, it wins.
  const pairingLive = pairing ? stream[pairing.id] : undefined;

  // Compared against currentUser explicitly: an unassigned thread and a
  // missing user would otherwise both be undefined and read as a match,
  // handing the delete button to a logged-out session.
  const canPurge =
    isAdmin || (!!currentUser && selected?.assignedUserId === currentUser.id);

  // mutateAsync rather than mutate, so the composer learns whether the reply
  // left before it throws away what the CS typed. The rejection is caught and
  // turned into a false: useSendCsMessage has already shown the reason, and an
  // uncaught rejection here would only add a console error on top of it.
  // Leaving a thread drops a quote started in it. The quote belongs to that
  // conversation — the API refuses it anywhere else — so carrying it across
  // would fail the send with a message a CS could do nothing about.
  // On a phone the panes take turns, so opening one is a step the back button
  // should undo. On a desktop nothing moves and the existing replace stands:
  // switching threads must not leave Back walking through every one of them.
  const layout = inboxLayout(screens, selectedId, searchParams.get("panel"));
  const showsPane = (pane: InboxPane) => layout.columns || layout.pane === pane;
  const paneWidth = (column: number) =>
    layout.columns ? { width: column } : { flex: 1, minWidth: 0 };

  const handleSelect = (id: string) => {
    patchParams({ conversation: id, panel: undefined }, !layout.columns);
    setReplyTo(undefined);
  };

  const handleSend = async (body: string): Promise<boolean> => {
    if (!selected) return false;
    try {
      await sendMessage.mutateAsync({
        conversationId: selected.id,
        body,
        replyToId: replyTo?.id,
      });
      setReplyTo(undefined);
      return true;
    } catch {
      return false;
    }
  };

  const handleAttach = async (
    file: File,
    caption: string,
  ): Promise<boolean> => {
    if (!selected) return false;
    try {
      await sendMedia.mutateAsync({
        conversationId: selected.id,
        file,
        caption,
        replyToId: replyTo?.id,
      });
      setReplyTo(undefined);
      return true;
    } catch {
      return false;
    }
  };

  const handleTakeOver = () => {
    if (!selected || !currentUser) return;
    assignConversation.mutate({
      conversationId: selected.id,
      userId: currentUser.id,
    });
  };

  return {
    currentUser,
    isAdmin,
    screens,
    numbersOpen,
    setNumbersOpen,
    pairing,
    setPairing,
    pairingLive,
    quickRepliesOpen,
    setQuickRepliesOpen,
    view,
    setView,
    setSearch,
    replyTo,
    setReplyTo,
    stream,
    csTyping,
    setTyping,
    push,
    conversations,
    selected,
    selectedId,
    historyQuery,
    usersQuery,
    onlineQuery,
    quickRepliesQuery,
    accountsQuery,
    accounts,
    holderNames,
    broadcast,
    sendMessage,
    sendMedia,
    assignConversation,
    connectAccount,
    createAccount,
    disconnectAccount,
    deleteAccount,
    deleteMessage,
    clearConversation,
    clearAccountMessages,
    clearInbox,
    notice,
    canPurge,
    layout,
    showsPane,
    paneWidth,
    navigate,
    patchParams,
    handleSelect,
    handleSend,
    handleAttach,
    handleTakeOver,
  };
}
