import { useState } from "react";
import { Button, Empty, Space } from "antd";
import { ThunderboltOutlined } from "@ant-design/icons";
import { useAuthStore } from "@/application/stores";
import {
  useAssignConversation,
  useConnectWaAccount,
  useDisconnectWaAccount,
  useCsConversations,
  useCsHistory,
  useCsQuickReplies,
  useCsStream,
  useSendCsMedia,
  useSendCsMessage,
  useUsers,
  useWaAccounts,
} from "@/application/hooks";
import { UserRole } from "@/domain/entities";
import type { CsMessage, User } from "@/domain/entities";
import { PageHeader } from "@/presentation/components/common";
import { ConversationList } from "@/presentation/components/cs/ConversationList";
import { ThreadPane } from "@/presentation/components/cs/ThreadPane";
import { CustomerPanel } from "@/presentation/components/cs/CustomerPanel";
import { WaConnectionBadge } from "@/presentation/components/cs/WaConnectionBadge";
import { WaPairingModal } from "@/presentation/components/cs/WaPairingModal";
import {
  InboxFilterBar,
  filterFor,
  type InboxView,
} from "@/presentation/components/cs/InboxFilterBar";
import { colors } from "@/shared/theme/colors";
import { QuickReplyManagerModal } from "@/presentation/components/cs/QuickReplyManagerModal";

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
 * what was said, and who they are. useCsStream is called once here — it also
 * carries the WhatsApp connection state, so the badge and the pairing modal
 * both read it from this page instead of opening a second connection.
 */
export function CsInboxPage() {
  const currentUser = useAuthStore((state) => state.user);
  const isAdmin = currentUser?.role === UserRole.ADMIN;

  const [selectedId, setSelectedId] = useState<string>();
  const [pairingOpen, setPairingOpen] = useState(false);
  const [quickRepliesOpen, setQuickRepliesOpen] = useState(false);
  // "Semua" by default, not "Milik saya": a CS opening the inbox needs to see
  // what nobody has picked up, not only what is already theirs.
  const [view, setView] = useState<InboxView>("semua");
  const [search, setSearch] = useState("");
  const [replyTo, setReplyTo] = useState<CsMessage>();

  const { waStatus, pairingCode } = useCsStream();
  const conversationsQuery = useCsConversations(filterFor(view, search));
  const historyQuery = useCsHistory(selectedId);
  const usersQuery = useUsers();
  const quickRepliesQuery = useCsQuickReplies();
  // Reading the account list is open to the whole inbox team, not just an
  // admin — a connection down for hours produces no live SSE event, so this
  // initial fetch is what makes the badge honest for a CS or technician too.
  const accountsQuery = useWaAccounts();

  const sendMessage = useSendCsMessage();
  const sendMedia = useSendCsMedia();
  const assignConversation = useAssignConversation();
  const connectAccount = useConnectWaAccount();
  const disconnectAccount = useDisconnectWaAccount();

  const conversations = conversationsQuery.data ?? [];
  const selected = conversations.find((c) => c.id === selectedId);
  const holderNames = holderNameMap(usersQuery.data ?? []);
  const account = accountsQuery.data?.[0];
  // A live status from this session's own stream is more current than the
  // admin-only fetch it started from — once one arrives, it wins.
  const connectionStatus = waStatus ?? account?.status;

  // mutateAsync rather than mutate, so the composer learns whether the reply
  // left before it throws away what the CS typed. The rejection is caught and
  // turned into a false: useSendCsMessage has already shown the reason, and an
  // uncaught rejection here would only add a console error on top of it.
  // Leaving a thread drops a quote started in it. The quote belongs to that
  // conversation — the API refuses it anywhere else — so carrying it across
  // would fail the send with a message a CS could do nothing about.
  const handleSelect = (id: string) => {
    setSelectedId(id);
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
        height: "calc(100vh - 96px)",
      }}
    >
      <PageHeader
        title="CS Inbox"
        description="Satu nomor WhatsApp, dijawab bersama satu tim"
        extra={
          <Space>
            {isAdmin && (
              <Button
                icon={<ThunderboltOutlined />}
                onClick={() => setQuickRepliesOpen(true)}
              >
                Balasan Cepat
              </Button>
            )}
            <WaConnectionBadge
              status={connectionStatus}
              onOpenPairing={isAdmin ? () => setPairingOpen(true) : undefined}
            />
          </Space>
        }
      />

      <div style={{ display: "flex", gap: 12, flex: 1, minHeight: 0 }}>
        <div
          style={{
            ...panel,
            width: 340,
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
              selectedId={selectedId}
              holderNames={holderNames}
              currentUserId={currentUser?.id ?? ""}
              onSelect={handleSelect}
            />
          </div>
        </div>

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
          />
        </div>

        <div
          style={{
            ...panel,
            width: 320,
            overflowY: "auto",
            padding: 14,
          }}
        >
          {selected ? (
            <CustomerPanel conversation={selected} />
          ) : (
            <Empty
              image={Empty.PRESENTED_IMAGE_SIMPLE}
              description="Data pelanggan muncul di sini"
            />
          )}
        </div>
      </div>

      {isAdmin && (
        <QuickReplyManagerModal
          open={quickRepliesOpen}
          onClose={() => setQuickRepliesOpen(false)}
          quickReplies={quickRepliesQuery.data ?? []}
        />
      )}

      {isAdmin && (
        <WaPairingModal
          open={pairingOpen}
          onClose={() => setPairingOpen(false)}
          status={connectionStatus}
          pairingCode={pairingCode}
          accountId={account?.id}
          connectedNumber={account?.jid?.split("@")[0]}
          connecting={connectAccount.isPending}
          onConnect={(phone) =>
            account && connectAccount.mutate({ id: account.id, phone })
          }
          disconnecting={disconnectAccount.isPending}
          onDisconnect={() => account && disconnectAccount.mutate(account.id)}
        />
      )}
    </div>
  );
}

export default CsInboxPage;
