import { useState } from "react";
import { Button, Empty, Space, Spin } from "antd";
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
import type { User } from "@/domain/entities";
import { PageHeader } from "@/presentation/components/common";
import { ConversationList } from "@/presentation/components/cs/ConversationList";
import { MessageThread } from "@/presentation/components/cs/MessageThread";
import { MessageComposer } from "@/presentation/components/cs/MessageComposer";
import { CustomerPanel } from "@/presentation/components/cs/CustomerPanel";
import { WaConnectionBadge } from "@/presentation/components/cs/WaConnectionBadge";
import { WaPairingModal } from "@/presentation/components/cs/WaPairingModal";
import { QuickReplyManagerModal } from "@/presentation/components/cs/QuickReplyManagerModal";
import { TransferPicker } from "@/presentation/components/cs/TransferPicker";

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

  const { waStatus, pairingCode } = useCsStream();
  const conversationsQuery = useCsConversations();
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
  const handleSend = async (body: string): Promise<boolean> => {
    if (!selected) return false;
    try {
      await sendMessage.mutateAsync({ conversationId: selected.id, body });
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
      });
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

      <div style={{ display: "flex", gap: 16, flex: 1, minHeight: 0 }}>
        <div
          style={{
            width: 300,
            overflowY: "auto",
            borderRight: "1px solid #27272a",
          }}
        >
          <ConversationList
            conversations={conversations}
            selectedId={selectedId}
            holderNames={holderNames}
            currentUserId={currentUser?.id ?? ""}
            onSelect={setSelectedId}
          />
        </div>

        <div
          style={{
            flex: 1,
            display: "flex",
            flexDirection: "column",
            minWidth: 0,
          }}
        >
          {selected ? (
            <>
              <div style={{ flex: 1, overflowY: "auto", padding: "0 8px" }}>
                {historyQuery.isLoading ? (
                  <Spin />
                ) : (
                  <MessageThread
                    messages={historyQuery.data ?? []}
                    onRetry={handleSend}
                  />
                )}
              </div>
              <div
                style={{
                  padding: "8px 8px 0",
                  display: "flex",
                  justifyContent: "flex-end",
                }}
              >
                <TransferPicker
                  users={usersQuery.data ?? []}
                  holderId={selected.assignedUserId}
                  transferring={assignConversation.isPending}
                  onTransfer={(userId) =>
                    assignConversation.mutate({
                      conversationId: selected.id,
                      userId,
                    })
                  }
                />
              </div>
              <div style={{ padding: 8 }}>
                <MessageComposer
                  conversation={selected}
                  currentUserId={currentUser?.id ?? ""}
                  holderName={
                    selected.assignedUserId
                      ? holderNames[selected.assignedUserId] ?? "pengguna lain"
                      : ""
                  }
                  onSend={handleSend}
                  onTakeOver={handleTakeOver}
                  onAttach={handleAttach}
                  quickReplies={quickRepliesQuery.data ?? []}
                  sending={sendMessage.isPending}
                  attaching={sendMedia.isPending}
                />
              </div>
            </>
          ) : (
            <Empty description="Pilih percakapan" style={{ marginTop: 80 }} />
          )}
        </div>

        <div
          style={{
            width: 300,
            overflowY: "auto",
            borderLeft: "1px solid #27272a",
            padding: "0 12px",
          }}
        >
          {selected && <CustomerPanel conversation={selected} />}
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
