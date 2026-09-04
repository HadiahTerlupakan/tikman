import { useEffect, useRef, useState } from "react";
import { Alert, Button, Input, Space, Upload, message } from "antd";
import {
  CloseOutlined,
  PaperClipOutlined,
  SendOutlined,
} from "@ant-design/icons";
import { colors } from "@/shared/theme/colors";
import type {
  CsConversation,
  CsMessage,
  CsQuickReply,
} from "@/domain/entities";
import { CS_MEDIA_ACCEPT, attachmentRejection } from "@/shared/config/csMedia";
import { QuickReplyPicker } from "./QuickReplyPicker";
import { QuotedBlock } from "./QuotedBlock";

const { TextArea } = Input;

/** How often the "typing…" line is refreshed while a CS keeps writing. A CS
 * types faster than WhatsApp needs to hear about it, and the line looks
 * identical whether it was refreshed once a keystroke or once every few
 * seconds. */
const TYPING_REFRESH_MS = 5_000;

/** How long a CS can stop mid-word before the line comes down. Shorter than
 * the stream's own expiry, so the customer stops seeing it because the CS
 * stopped rather than because a timer ran out. */
const TYPING_IDLE_MS = 3_000;

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
  onAttach,
  quickReplies = [],
  sending = false,
  attaching = false,
  replyTo,
  onCancelReply,
  onTypingChange,
}: MessageComposerProps) {
  const [text, setText] = useState("");
  const isHolder = conversation.assignedUserId === currentUserId;

  // Which thread the customer currently sees a line on, null when none does.
  const typingFor = useRef<string | null>(null);
  const lastSignalAt = useRef(0);
  const idle = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // The latest callback, so the unmount cleanup below does not have to hold a
  // stale one — the thread it names travels with the call, not with the
  // closure.
  const notify = useRef(onTypingChange);
  notify.current = onTypingChange;

  const signalTyping = (typing: boolean) => {
    clearTimeout(idle.current);
    if (typing) {
      idle.current = setTimeout(() => signalTyping(false), TYPING_IDLE_MS);
      const unchanged = typingFor.current === conversation.id;
      if (unchanged && Date.now() - lastSignalAt.current < TYPING_REFRESH_MS) {
        return;
      }
      typingFor.current = conversation.id;
      lastSignalAt.current = Date.now();
      notify.current(conversation.id, true);
      return;
    }
    const showing = typingFor.current;
    if (!showing) return;
    typingFor.current = null;
    notify.current(showing, false);
  };

  useEffect(() => {
    // Switching threads mid-word clears the line on the thread that was left.
    // typingFor still names it here: this runs after the render that brought
    // the new thread in, and nothing has signalled on it yet.
    if (typingFor.current && typingFor.current !== conversation.id) {
      const left = typingFor.current;
      typingFor.current = null;
      clearTimeout(idle.current);
      notify.current(left, false);
    }
  }, [conversation.id]);

  useEffect(
    () => () => {
      clearTimeout(idle.current);
      if (typingFor.current) {
        notify.current(typingFor.current, false);
      }
    },
    [],
  );

  if (!isHolder) {
    return (
      <Space
        direction="vertical"
        style={{
          width: "100%",
          padding: "10px 12px",
          borderTop: `1px solid ${colors.border}`,
          background: colors.surface,
        }}
      >
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
    signalTyping(false);
    if (await onSend(body)) {
      setText("");
    }
  };

  // beforeUpload always answers false: antd would otherwise post the file
  // itself, and this endpoint needs the session cookie and the caption that
  // is sitting in the box.
  const handleAttach = async (file: File) => {
    const rejection = attachmentRejection(file);
    if (rejection) {
      message.error(rejection);
      return false;
    }
    signalTyping(false);
    if (await onAttach(file, text.trim())) {
      setText("");
    }
    return false;
  };

  return (
    <div
      style={{
        padding: "10px 12px",
        borderTop: `1px solid ${colors.border}`,
        background: colors.surface,
      }}
    >
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

      <div style={{ display: "flex", alignItems: "flex-end", gap: 6 }}>
        <QuickReplyPicker quickReplies={quickReplies} onPick={setText} />
        <Upload
          accept={CS_MEDIA_ACCEPT}
          showUploadList={false}
          beforeUpload={handleAttach}
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
          loading={sending}
          disabled={!text.trim()}
          aria-label="Kirim"
          title="Kirim"
        />
      </div>
    </div>
  );
}
