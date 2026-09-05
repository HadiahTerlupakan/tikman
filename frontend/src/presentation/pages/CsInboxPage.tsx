import { useEffect, useState } from "react";
import {
  useNavigate,
  useOutletContext,
  useSearchParams,
} from "react-router-dom";
import { Grid, Typography } from "antd";
import { Empty } from "antd";
import { useAuthStore } from "@/application/stores";
import { claimPresence } from "@/infrastructure/firebase/presence";
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
import { UserRole } from "@/domain/entities";
import type { CsMessage, User, WaAccount } from "@/domain/entities";
import { PageHeader } from "@/presentation/components/common";
import { ConversationList } from "@/presentation/components/cs/ConversationList";
import { ThreadPane } from "@/presentation/components/cs/ThreadPane";
import { CustomerPanel } from "@/presentation/components/cs/CustomerPanel";
import { CsTeamPanel } from "@/presentation/components/cs/CsTeamPanel";
import { WaPairingModal } from "@/presentation/components/cs/WaPairingModal";
import { WaNumbersModal } from "@/presentation/components/cs/WaNumbersModal";
import { InboxHeaderActions } from "@/presentation/components/cs/InboxHeaderActions";
import { BroadcastModal } from "@/presentation/components/cs/BroadcastModal";
import {
  InboxFilterBar,
  filterFor,
  isInboxView,
  type InboxView,
} from "@/presentation/components/cs/InboxFilterBar";
import { fullHeightPage } from "@/presentation/components/layout/layoutPadding";
import { inboxLayout, type InboxPane } from "./inboxLayout";
import { colors } from "@/shared/theme/colors";
import { QuickReplyManagerModal } from "@/presentation/components/cs/QuickReplyManagerModal";
import type { AppLayoutContext } from "@/presentation/components/layout/AppLayout";

const { Text } = Typography;

// One shape for all three columns: without it they read as content floating on
// the page rather than as panes of one screen.
const panel = {
  background: colors.surface,
  border: `1px solid ${colors.border}`,
  borderRadius: 10,
  overflow: "hidden",
} as const;

function holderNameMap(users: User[]): Record<string, string> {
  return Object.fromEntries(users.map((u) => [u.id, u.username]));
}

/**
 * Three columns share one WhatsApp number across a CS team: who to talk to,
 * what was said, and who they are. The event stream (also driving the navbar
 * badge) runs once in AppLayout and reaches this page via useOutletContext,
 * so the pairing modal reads the same connection state instead of a second one.
 */
export function CsInboxPage() {
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

  // Only this page claims presence — the same rule the SSE stream's
  // ?presence=1 followed: someone reading the OLT map is not at the inbox.
  useEffect(() => {
    let release: (() => Promise<void>) | undefined;
    let cancelled = false;

    claimPresence()
      .then((r) => {
        if (cancelled) {
          void r();
          return;
        }
        release = r;
      })
      .catch((error) => console.warn("Could not claim presence", error));

    return () => {
      cancelled = true;
      void release?.();
    };
  }, []);

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

  return (
    <div
      style={{
        display: "flex",
        flexDirection: "column",
        height: fullHeightPage(screens),
      }}
    >
      <PageHeader
        title="CS Inbox"
        description="Satu nomor WhatsApp, dijawab bersama satu tim"
        extra={
          <InboxHeaderActions
            isAdmin={isAdmin}
            accounts={accountsQuery.data}
            stream={stream}
            pushPermission={push.permission}
            pushRequesting={push.requesting}
            onEnablePush={push.enable}
            onOpenQuickReplies={() => setQuickRepliesOpen(true)}
            onOpenNumbers={() => setNumbersOpen(true)}
            onOpenBroadcast={broadcast.onOpen}
          />
        }
      />

      <div style={{ display: "flex", gap: 12, flex: 1, minHeight: 0 }}>
        {showsPane("list") && (
          <div
            style={{
              ...panel,
              ...paneWidth(340),
              display: "flex",
              flexDirection: "column",
            }}
          >
            <InboxFilterBar
              view={view}
              onViewChange={setView}
              onSearchChange={setSearch}
            />
            <div style={{ flex: 1, overflowY: "auto" }}>
              <ConversationList
                conversations={conversations}
                typingIn={csTyping}
                selectedId={selectedId}
                holderNames={holderNames}
                currentUserId={currentUser?.id ?? ""}
                onSelect={handleSelect}
              />
            </div>
          </div>
        )}

        {showsPane("thread") && (
          <div
            style={{
              ...panel,
              flex: 1,
              display: "flex",
              flexDirection: "column",
              minWidth: 0,
            }}
          >
            <ThreadPane
              conversation={selected}
              customerTyping={!!selected && !!csTyping[selected.id]}
              onTypingChange={setTyping}
              messages={historyQuery.data ?? []}
              loading={historyQuery.isLoading}
              currentUserId={currentUser?.id ?? ""}
              holderNames={holderNames}
              users={usersQuery.data ?? []}
              quickReplies={quickRepliesQuery.data ?? []}
              replyTo={replyTo}
              sending={sendMessage.isPending}
              attaching={sendMedia.isPending}
              transferring={assignConversation.isPending}
              clearing={clearConversation.isPending}
              canPurge={canPurge}
              onSend={handleSend}
              onAttach={handleAttach}
              onTakeOver={handleTakeOver}
              onTransfer={(userId) =>
                selected &&
                assignConversation.mutate({
                  conversationId: selected.id,
                  userId,
                })
              }
              onReply={setReplyTo}
              onCancelReply={() => setReplyTo(undefined)}
              onDeleteMessage={(message) =>
                selected &&
                deleteMessage.mutate({
                  messageId: message.id,
                  conversationId: selected.id,
                })
              }
              onClearThread={() =>
                selected && clearConversation.mutate(selected.id)
              }
              onBack={layout.columns ? undefined : () => navigate(-1)}
              onOpenCustomer={
                layout.columns
                  ? undefined
                  : () => patchParams({ panel: "customer" }, true)
              }
            />
          </div>
        )}

        {showsPane("customer") && (
          <div
            style={{
              ...panel,
              ...paneWidth(320),
              display: "flex",
              flexDirection: "column",
            }}
          >
            <div style={{ flex: 1, overflowY: "auto", padding: 14 }}>
              {selected ? (
                <CustomerPanel conversation={selected} />
              ) : (
                <Empty
                  image={Empty.PRESENTED_IMAGE_SIMPLE}
                  description="Data pelanggan muncul di sini"
                />
              )}
            </div>

            {/* Fixed height rather than shared: who is at the inbox is
                ambient, and letting it grow would push the subscriber's
                details — the reason the column exists — off the screen. */}
            <div
              style={{
                borderTop: `1px solid ${colors.border}`,
                maxHeight: 200,
                overflowY: "auto",
                flexShrink: 0,
              }}
            >
              {!onlineQuery.connected && (
                <Text
                  style={{
                    display: "block",
                    padding: "8px 14px 0",
                    color: colors.textMuted,
                    fontSize: 11,
                  }}
                >
                  Terputus — daftar ini mungkin sudah tidak akurat
                </Text>
              )}
              <CsTeamPanel
                users={usersQuery.data ?? []}
                online={onlineQuery.data ?? []}
                currentUserId={currentUser?.id}
              />
            </div>
          </div>
        )}
      </div>

      {isAdmin && (
        <QuickReplyManagerModal
          open={quickRepliesOpen}
          onClose={() => setQuickRepliesOpen(false)}
          quickReplies={quickRepliesQuery.data ?? []}
        />
      )}

      {isAdmin && (
        <WaNumbersModal
          open={numbersOpen}
          onClose={() => setNumbersOpen(false)}
          accounts={accounts}
          stream={stream}
          adding={createAccount.isPending}
          busy={
            deleteAccount.isPending ||
            clearInbox.isPending ||
            clearAccountMessages.isPending
          }
          onAdd={(label) => createAccount.mutate(label)}
          onPair={setPairing}
          onClearMessages={(account) => clearAccountMessages.mutate(account.id)}
          onDelete={(account) => deleteAccount.mutate(account.id)}
          onClearInbox={() => clearInbox.mutate()}
        />
      )}

      <BroadcastModal {...broadcast} />

      {isAdmin && pairing && (
        <WaPairingModal
          open
          onClose={() => setPairing(undefined)}
          status={pairingLive?.waStatus ?? pairing.status}
          pairingCode={pairingLive?.pairingCode}
          accountId={pairing.id}
          connectedNumber={pairing.jid?.split("@")[0]}
          connecting={connectAccount.isPending}
          onConnect={(phone) =>
            connectAccount.mutate({ id: pairing.id, phone })
          }
          disconnecting={disconnectAccount.isPending}
          onDisconnect={() => disconnectAccount.mutate(pairing.id)}
        />
      )}
    </div>
  );
}

export default CsInboxPage;
