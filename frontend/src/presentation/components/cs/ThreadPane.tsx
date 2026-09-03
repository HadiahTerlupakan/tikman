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
import { MessageComposer } from "./MessageComposer";
import { MessageThread } from "./MessageThread";
import { ThreadHeader } from "./ThreadHeader";
import { TransferPicker } from "./TransferPicker";

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
}

/**
 * One conversation being worked: who it is with, what was said, and the box to
 * answer in. Everything here is gated on holding the thread — a CS who does
 * not hold it reads freely but is offered no way to reply, because the API
 * would refuse the send and there is nothing they could do about it.
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
}: ThreadPaneProps) {
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
        onClear={canPurge ? onClearThread : undefined}
        clearing={clearing}
      />

      <div
        style={{
          flex: 1,
          overflowY: "auto",
          padding: "12px 14px",
          background: chatBackdropColor,
          backgroundImage: chatBackdropImage,
          backgroundSize: chatBackdropSize,
        }}
      >
        {loading ? (
          <Spin />
        ) : (
          <MessageThread
            messages={messages}
            onRetry={onSend}
            onReply={isHolder ? onReply : undefined}
            onDelete={canPurge ? onDeleteMessage : undefined}
          />
        )}
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
        onSend={onSend}
        onTakeOver={onTakeOver}
        onAttach={onAttach}
        quickReplies={quickReplies}
        sending={sending}
        attaching={attaching}
        replyTo={replyTo}
        onCancelReply={onCancelReply}
      />
    </>
  );
}
