import { useState, type ClipboardEvent } from "react";
import { Button, Input, Typography, Upload, message } from "antd";
import {
  CloseOutlined,
  PaperClipOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { colors } from "@/shared/theme/colors";
import { useLinkPreview } from "@/application/hooks";
import { LinkPreviewCard } from "./LinkPreviewCard";
import type {
  CsConversation,
  CsMessage,
  CsQuickReply,
} from "@/domain/entities";
import { CS_MEDIA_ACCEPT, attachmentRejection } from "@/shared/config/csMedia";
import { mayReply } from "./holding";
import { PendingAttachment } from "./PendingAttachment";
import { QuickReplyPicker } from "./QuickReplyPicker";
import { useTypingSignal } from "./useTypingSignal";
import { QuotedBlock } from "./QuotedBlock";
import { TakeOverNotice } from "./TakeOverNotice";

const { TextArea } = Input;
const { Text } = Typography;

interface MessageComposerProps {
  conversation: CsConversation;
  currentUserId: string;
  holderName: string;
  /** Answers whether the reply actually left. A false clears nothing: the CS
   * still has what they typed, and the hook has already said why it failed. */
  onSend: (body: string) => Promise<boolean>;
  onTakeOver: () => void;
  /** Sends one attachment, carrying whatever caption is in the box with it. */
  onAttach: (file: File, caption: string) => Promise<boolean>;
  quickReplies?: CsQuickReply[];
  sending?: boolean;
  attaching?: boolean;
  /** The message being answered, shown above the box so a CS sees what the
   * customer will see before it is sent. */
  replyTo?: CsMessage;
  onCancelReply?: () => void;
  /** Raises or clears the "typing…" line on the customer's phone. The thread
   * is named rather than assumed, so leaving one mid-word clears the line on
   * the thread that was left, not on the one just opened. */
  onTypingChange: (conversationId: string, typing: boolean) => void;
}

/**
 * The reply box, for a thread this CS may answer: their own, or one nobody
 * holds yet — where the first reply is what makes them its holder. Anyone else
 * is shown who is serving the customer instead (see TakeOverNotice).
 */
export function MessageComposer({
  conversation,
  currentUserId,
  holderName,
  onSend,
  onTakeOver,
  onAttach,
  quickReplies = [],
  sending = false,
  attaching = false,
  replyTo,
  onCancelReply,
  onTypingChange,
}: MessageComposerProps) {
  const [text, setText] = useState("");
  const linkPreview = useLinkPreview(text);
  // The attachment waiting to be sent, kept with the thread it was added in:
  // shown in any other, it would go to the wrong customer.
  const [pending, setPending] = useState<{
    file: File;
    conversationId: string;
  } | null>(null);
  const staged =
    pending?.conversationId === conversation.id ? pending.file : null;

  const signalTyping = useTypingSignal(conversation.id, onTypingChange);

  if (!mayReply(conversation, currentUserId)) {
    return (
      <TakeOverNotice
        conversation={conversation}
        holderName={holderName}
        onTakeOver={onTakeOver}
      />
    );
  }

  // With an attachment waiting, the box is its caption and sending sends both.
  const handleSend = async () => {
    const body = text.trim();
    if (!staged && !body) return;
    signalTyping(false);
    const sent = staged ? await onAttach(staged, body) : await onSend(body);
    if (sent) {
      setText("");
      setPending(null);
    }
  };

  // Picked or pasted, a file waits for a look before it leaves, the way
  // WhatsApp shows one: sending the moment a file was picked left no chance to
  // take back the wrong one, and a paste lands even more easily by mistake.
  const stage = (file: File) => {
    const rejection = attachmentRejection(file);
    if (rejection) {
      message.error(rejection);
      return;
    }
    setPending({ file, conversationId: conversation.id });
  };

  // Only a paste that carries a file is taken over; text is left to the box.
  const handlePaste = (event: ClipboardEvent<HTMLTextAreaElement>) => {
    const file = event.clipboardData.files[0];
    if (!file) return;
    event.preventDefault();
    stage(file);
  };

  return (
    <div
      style={{
        padding: "10px 12px",
        borderTop: `1px solid ${colors.border}`,
        background: colors.surface,
      }}
    >
      {conversation.status === "unassigned" && (
        <Text
          type="secondary"
          style={{ display: "block", fontSize: 12, marginBottom: 6 }}
        >
          Belum dipegang — balasan Anda akan menjadikan Anda yang melayani
          percakapan ini.
        </Text>
      )}

      {replyTo && (
        <div
          style={{
            display: "flex",
            alignItems: "center",
            gap: 6,
            marginBottom: 6,
          }}
        >
          <div style={{ flex: 1, minWidth: 0 }}>
            <QuotedBlock
              quoted={{
                id: replyTo.id,
                direction: replyTo.direction,
                kind: replyTo.kind,
                body: replyTo.body,
                mediaFilename: replyTo.mediaFilename,
              }}
              authorLabel={replyTo.direction === "out" ? "Anda" : "Pelanggan"}
            />
          </div>
          <Button
            type="text"
            size="small"
            icon={<CloseOutlined />}
            onClick={onCancelReply}
            aria-label="Batalkan balasan"
            title="Batalkan balasan"
          />
        </div>
      )}

      {linkPreview.preview && (
        <LinkPreviewCard
          preview={linkPreview.preview}
          onDismiss={linkPreview.dismiss}
        />
      )}

      {staged && (
        <PendingAttachment file={staged} onCancel={() => setPending(null)} />
      )}

      <div style={{ display: "flex", alignItems: "flex-end", gap: 6 }}>
        <QuickReplyPicker quickReplies={quickReplies} onPick={setText} />
        {/* beforeUpload answers false: antd would otherwise post the file
            itself, and this endpoint needs the session cookie and a caption. */}
        <Upload
          accept={CS_MEDIA_ACCEPT}
          showUploadList={false}
          beforeUpload={(file) => {
            stage(file);
            return false;
          }}
        >
          <Button
            type="text"
            icon={<PaperClipOutlined />}
            loading={attaching}
            title="Lampirkan berkas"
            aria-label="Lampirkan berkas"
          />
        </Upload>
        <TextArea
          value={text}
          onChange={(e) => {
            setText(e.target.value);
            signalTyping(e.target.value.trim().length > 0);
          }}
          onBlur={() => signalTyping(false)}
          onPaste={handlePaste}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
          autoSize={{ minRows: 1, maxRows: 5 }}
          placeholder="Tulis balasan"
          variant="filled"
          style={{ borderRadius: 18, padding: "6px 12px", resize: "none" }}
        />
        <Button
          type="primary"
          shape="circle"
          icon={<SendOutlined />}
          onClick={handleSend}
          loading={sending || attaching}
          disabled={!text.trim() && !staged}
          aria-label="Kirim"
          title="Kirim"
        />
      </div>
    </div>
  );
}
