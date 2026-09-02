import { useState } from "react";
import { Alert, Button, Input, Space } from "antd";
import type { CsConversation, CsQuickReply } from "@/domain/entities";
import { QuickReplyPicker } from "./QuickReplyPicker";

const { TextArea } = Input;

interface MessageComposerProps {
  conversation: CsConversation;
  currentUserId: string;
  holderName: string;
  /** Answers whether the reply actually left. A false clears nothing: the CS
   * still has what they typed, and the hook has already said why it failed. */
  onSend: (body: string) => Promise<boolean>;
  onTakeOver: () => void;
  quickReplies?: CsQuickReply[];
  sending?: boolean;
}

/**
 * A greyed-out send button with no reason reads as a broken page. When
 * someone else holds the thread, this says who — and offers the way in:
 * taking over is allowed and audited, not a dead end a CS has to ask around
 * about.
 */
export function MessageComposer({
  conversation,
  currentUserId,
  holderName,
  onSend,
  onTakeOver,
  quickReplies = [],
  sending = false,
}: MessageComposerProps) {
  const [text, setText] = useState("");
  const isHolder = conversation.assignedUserId === currentUserId;

  if (!isHolder) {
    return (
      <Space direction="vertical" style={{ width: "100%" }}>
        <Alert
          type="info"
          showIcon
          message={
            conversation.assignedUserId
              ? `Dipegang ${holderName} — ambil alih dulu untuk membalas`
              : "Belum dipegang siapa pun — ambil alih untuk membalas"
          }
        />
        <Button onClick={onTakeOver}>Ambil alih</Button>
      </Space>
    );
  }

  const handleSend = async () => {
    const body = text.trim();
    if (!body) return;
    if (await onSend(body)) {
      setText("");
    }
  };

  return (
    <Space.Compact style={{ width: "100%" }}>
      <QuickReplyPicker quickReplies={quickReplies} onPick={setText} />
      <TextArea
        value={text}
        onChange={(e) => setText(e.target.value)}
        onPressEnter={(e) => {
          if (!e.shiftKey) {
            e.preventDefault();
            handleSend();
          }
        }}
        autoSize={{ minRows: 1, maxRows: 4 }}
        placeholder="Tulis balasan..."
      />
      <Button
        type="primary"
        onClick={handleSend}
        loading={sending}
        disabled={!text.trim()}
      >
        Kirim
      </Button>
    </Space.Compact>
  );
}
