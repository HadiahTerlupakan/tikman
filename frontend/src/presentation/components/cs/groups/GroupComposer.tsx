import { useState, type ClipboardEvent } from "react";
import { Alert, Button, Input, Upload, message as toast } from "antd";
import {
  CloseOutlined,
  PaperClipOutlined,
  SendOutlined,
} from "@ant-design/icons";
import type { WaGroup, WaGroupMessage } from "@/domain/entities";
import { CS_MEDIA_ACCEPT, attachmentRejection } from "@/shared/config/csMedia";
import { colors } from "@/shared/theme/colors";
import { PendingAttachment } from "../PendingAttachment";
import { QuotedBlock } from "../QuotedBlock";
import { sendBlockedReason, senderLabel } from "./groupSummary";

const { TextArea } = Input;

interface GroupComposerProps {
  group: WaGroup;
  /** Answers whether the message left; a false keeps what was typed. */
  onSend: (body: string) => Promise<boolean>;
  onAttach: (file: File, caption: string) => Promise<boolean>;
  sending: boolean;
  attaching: boolean;
  replyTo?: WaGroupMessage;
  onCancelReply: () => void;
}

function ReplyingTo({
  replyTo,
  onCancel,
}: {
  replyTo: WaGroupMessage;
  onCancel: () => void;
}) {
  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: 6, marginBottom: 6 }}
    >
      <div style={{ flex: 1, minWidth: 0 }}>
        <QuotedBlock quoted={replyTo} authorLabel={senderLabel(replyTo)} />
      </div>
      <Button
        type="text"
        size="small"
        icon={<CloseOutlined />}
        onClick={onCancel}
        aria-label="Batalkan balasan"
        title="Batalkan balasan"
      />
    </div>
  );
}

/**
 * The box for writing to a group. Where the group will not take a message from
 * this number — it left, or only admins may send — the box is replaced by the
 * reason, since the API would refuse the send anyway.
 */
export function GroupComposer({
  group,
  onSend,
  onAttach,
  sending,
  attaching,
  replyTo,
  onCancelReply,
}: GroupComposerProps) {
  const [text, setText] = useState("");
  // Kept with the group it was added in: shown in another, it would be posted
  // to the wrong group.
  const [pending, setPending] = useState<{ file: File; groupId: string }>();
  const staged = pending?.groupId === group.id ? pending.file : undefined;

  const blocked = sendBlockedReason(group);
  if (blocked) {
    return (
      <div
        style={{
          padding: "10px 12px",
          borderTop: `1px solid ${colors.border}`,
        }}
      >
        <Alert type="info" showIcon message={blocked} />
      </div>
    );
  }

  const handleSend = async () => {
    const body = text.trim();
    if (!staged && !body) return;
    const sent = staged ? await onAttach(staged, body) : await onSend(body);
    if (sent) {
      setText("");
      setPending(undefined);
    }
  };

  const stage = (file: File) => {
    const rejection = attachmentRejection(file);
    if (rejection) {
      toast.error(rejection);
      return;
    }
    setPending({ file, groupId: group.id });
  };

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
      {replyTo && <ReplyingTo replyTo={replyTo} onCancel={onCancelReply} />}
      {staged && (
        <PendingAttachment
          file={staged}
          onCancel={() => setPending(undefined)}
        />
      )}
      <div style={{ display: "flex", alignItems: "flex-end", gap: 6 }}>
        {/* false: antd would otherwise post the file itself, without the
            session cookie or the caption. */}
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
          onChange={(e) => setText(e.target.value)}
          onPaste={handlePaste}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
          autoSize={{ minRows: 1, maxRows: 5 }}
          placeholder="Tulis pesan ke grup"
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
