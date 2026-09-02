import { useState } from "react";
import { Empty, Spin } from "antd";
import { useAuthStore } from "@/application/stores";
import {
  useAssignConversation,
  useConnectWaAccount,
  useCsConversations,
  useCsHistory,
  useCsQuickReplies,
  useCsStream,
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

  const { waStatus, pairingCode } = useCsStream();
  const conversationsQuery = useCsConversations();
  const historyQuery = useCsHistory(selectedId);
  const usersQuery = useUsers();
  const quickRepliesQuery = useCsQuickReplies();
  const accountsQuery = useWaAccounts(isAdmin);

  const sendMessage = useSendCsMessage();
  const assignConversation = useAssignConversation();
  const connectAccount = useConnectWaAccount();

  const conversations = conversationsQuery.data ?? [];
  const selected = conversations.find((c) => c.id === selectedId);
  const holderNames = holderNameMap(usersQuery.data ?? []);
  const account = accountsQuery.data?.[0];
  // A live status from this session's own stream is more current than the
  // admin-only fetch it started from — once one arrives, it wins.
  const connectionStatus = waStatus ?? account?.status;

  const handleSend = (body: string) => {
    if (!selected) return;
    sendMessage.mutate({ conversationId: selected.id, body });
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
          <WaConnectionBadge
            status={connectionStatus}
            onOpenPairing={isAdmin ? () => setPairingOpen(true) : undefined}
          />
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
                  quickReplies={quickRepliesQuery.data ?? []}
                  sending={sendMessage.isPending}
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
        <WaPairingModal
          open={pairingOpen}
          onClose={() => setPairingOpen(false)}
          status={connectionStatus}
          pairingCode={pairingCode}
          connecting={connectAccount.isPending}
          onConnect={(phone) =>
            account && connectAccount.mutate({ id: account.id, phone })
          }
        />
      )}
    </div>
  );
}

export default CsInboxPage;
