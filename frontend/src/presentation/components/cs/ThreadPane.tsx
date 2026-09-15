import { Empty, Spin } from "antd";
import type {
  CsConversation,
  CsMessage,
  CsQuickReply,
  User,
} from "@/domain/entities";
import {
  chatBackdropColor,
  chatBackdropImage,
  chatBackdropSize,
} from "@/shared/theme/chatBackdrop";
import { mayReply } from "./holding";
import { MessageComposer } from "./MessageComposer";
import { MessageThread } from "./MessageThread";
import { ThreadHeader } from "./ThreadHeader";
import { TransferPicker } from "./TransferPicker";
import { useStickToBottom } from "./useStickToBottom";

interface ThreadPaneProps {
  /** Absent until a CS picks a thread out of the inbox. */
  conversation?: CsConversation;
  messages: CsMessage[];
  loading: boolean;
  currentUserId: string;
  holderNames: Record<string, string>;
  users: User[];
  quickReplies: CsQuickReply[];
  replyTo?: CsMessage;
  sending: boolean;
  attaching: boolean;
  transferring: boolean;
  clearing: boolean;
  /** True when the viewer may remove messages from this thread: its holder,
   * or an admin. */
  canPurge: boolean;
  onSend: (body: string) => Promise<boolean>;
  onAttach: (file: File, caption: string) => Promise<boolean>;
  onTakeOver: () => void;
  onTransfer: (userId: string) => void;
  onReply: (message: CsMessage) => void;
  onCancelReply: () => void;
  onDeleteMessage: (message: CsMessage) => void;
  onClearThread: () => void;
  /** Handed to ThreadHeader, and present only where the panes take turns. */
  onBack?: () => void;
  onOpenCustomer?: () => void;
  /** True while the customer is writing in this thread. */
  customerTyping: boolean;
  /** Raises or clears the "typing…" line on the customer's phone. */
  onTypingChange: (conversationId: string, typing: boolean) => void;
}

/**
 * One conversation being worked: who it is with, what was said, and the box to
 * answer in. Replying is gated on mayReply — a CS reads any thread freely, but
 * is offered a reply only on their own or one nobody holds, because the API
 * would refuse any other send and there is nothing they could do about it.
 * Handing the thread on stays with its holder.
 */
export function ThreadPane({
  conversation,
  messages,
  loading,
  currentUserId,
  holderNames,
  users,
  quickReplies,
  replyTo,
  sending,
  attaching,
  transferring,
  clearing,
  canPurge,
  onSend,
  onAttach,
  onTakeOver,
  onTransfer,
  onReply,
  onCancelReply,
  onDeleteMessage,
  onClearThread,
  onBack,
  onOpenCustomer,
  customerTyping,
  onTypingChange,
}: ThreadPaneProps) {
  const log = useStickToBottom(conversation?.id, messages, loading);

  if (!conversation) {
    return (
      <div
        style={{
          flex: 1,
          display: "flex",
          alignItems: "center",
          justifyContent: "center",
        }}
      >
        <Empty description="Pilih percakapan untuk mulai membalas" />
      </div>
    );
  }

  const isHolder = conversation.assignedUserId === currentUserId;

  return (
    <>
      <ThreadHeader
        conversation={conversation}
        holderName={
          conversation.assignedUserId
            ? holderNames[conversation.assignedUserId]
            : undefined
        }
        isHolder={isHolder}
        typing={customerTyping}
        onBack={onBack}
        onOpenCustomer={onOpenCustomer}
        onClear={canPurge ? onClearThread : undefined}
        clearing={clearing}
      />

      <div
        ref={log.containerRef}
        onScroll={log.onScroll}
        role="log"
        aria-label="Riwayat percakapan"
        style={{
          flex: 1,
          overflowY: "auto",
          padding: "12px 14px",
          background: chatBackdropColor,
          backgroundImage: chatBackdropImage,
          backgroundSize: chatBackdropSize,
        }}
      >
        <div ref={log.contentRef}>
          {loading ? (
            <Spin />
          ) : (
            <MessageThread
              messages={messages}
              onRetry={onSend}
              onReply={
                mayReply(conversation, currentUserId) ? onReply : undefined
              }
              onDelete={canPurge ? onDeleteMessage : undefined}
            />
          )}
        </div>
      </div>

      {isHolder && (
        <div
          style={{
            display: "flex",
            justifyContent: "flex-end",
            padding: "6px 12px 0",
          }}
        >
          <TransferPicker
            users={users}
            holderId={conversation.assignedUserId}
            transferring={transferring}
            onTransfer={onTransfer}
          />
        </div>
      )}

      <MessageComposer
        conversation={conversation}
        currentUserId={currentUserId}
        holderName={
          conversation.assignedUserId
            ? holderNames[conversation.assignedUserId] ?? "pengguna lain"
            : ""
        }
        onSend={(body) => {
          log.stick();
          return onSend(body);
        }}
        onTakeOver={onTakeOver}
        onAttach={(file, caption) => {
          log.stick();
          return onAttach(file, caption);
        }}
        quickReplies={quickReplies}
        sending={sending}
        attaching={attaching}
        replyTo={replyTo}
        onCancelReply={onCancelReply}
        onTypingChange={onTypingChange}
      />
    </>
  );
}
