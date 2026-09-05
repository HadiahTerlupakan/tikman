import { PageHeader } from "@/presentation/components/common";
import { ConversationList } from "@/presentation/components/cs/ConversationList";
import { ThreadPane } from "@/presentation/components/cs/ThreadPane";
import { CustomerPane } from "@/presentation/components/cs/CustomerPane";
import { WaPairingModal } from "@/presentation/components/cs/WaPairingModal";
import { WaNumbersModal } from "@/presentation/components/cs/WaNumbersModal";
import { InboxHeaderActions } from "@/presentation/components/cs/InboxHeaderActions";
import { BroadcastModal } from "@/presentation/components/cs/BroadcastModal";
import { InboxFilterBar } from "@/presentation/components/cs/InboxFilterBar";
import { QuickReplyManagerModal } from "@/presentation/components/cs/QuickReplyManagerModal";
import { fullHeightPage } from "@/presentation/components/layout/layoutPadding";
import { colors } from "@/shared/theme/colors";
import { useCsInbox } from "./useCsInbox";

// One shape for all three columns: without it they read as content floating on
// the page rather than as panes of one screen.
const panel = {
  background: colors.surface,
  border: `1px solid ${colors.border}`,
  borderRadius: 10,
  overflow: "hidden",
} as const;

/**
 * Three columns share one WhatsApp number across a CS team: who to talk to,
 * what was said, and who they are. The event stream (also driving the navbar
 * badge) runs once in AppLayout and reaches this page via useOutletContext,
 * so the pairing modal reads the same connection state instead of a second one.
 */
export function CsInboxPage() {
  const {
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
  } = useCsInbox();

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
            <CustomerPane
              conversation={selected}
              users={usersQuery.data ?? []}
              online={onlineQuery.data ?? []}
              currentUserId={currentUser?.id}
              notice={notice}
            />
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
