import { useState } from "react";
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
}: MessageComposerProps) {
  const [text, setText] = useState("");
  const isHolder = conversation.assignedUserId === currentUserId;

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
          onChange={(e) => setText(e.target.value)}
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
