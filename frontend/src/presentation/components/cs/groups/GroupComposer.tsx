import { useState, type ClipboardEvent } from "react";
import {
  Alert,
  Button,
  Dropdown,
  Mentions,
  Upload,
  message as toast,
} from "antd";
import {
  BarChartOutlined,
  CloseOutlined,
  PaperClipOutlined,
  SendOutlined,
} from "@ant-design/icons";
import type {
  WaGroup,
  WaGroupMessage,
  WaGroupParticipant,
  WaGroupPollRequest,
} from "@/domain/entities";
import { CS_MEDIA_ACCEPT, attachmentRejection } from "@/shared/config/csMedia";
import { colors } from "@/shared/theme/colors";
import { PendingAttachment } from "../PendingAttachment";
import { QuotedBlock } from "../QuotedBlock";
import { useTypingSignal } from "../useTypingSignal";
import { CreatePollModal } from "./CreatePollModal";
import { sendBlockedReason, senderLabel } from "./groupSummary";
import { mentionOptions, toWhatsAppMentions } from "./groupMentions";

interface GroupComposerProps {
  group: WaGroup;
  /** Who "@" offers. */
  participants: WaGroupParticipant[];
  /** Answers whether the message left; a false keeps what was typed. */
  onSend: (body: string, mentions: string[]) => Promise<boolean>;
  onAttach: (
    file: File,
    caption: string,
    mentions: string[],
  ) => Promise<boolean>;
  sending: boolean;
  attaching: boolean;
  replyTo?: WaGroupMessage;
  onCancelReply: () => void;
  /** Raises or clears our "sedang mengetik" in the named group. */
  onTypingChange?: (groupId: string, typing: boolean) => void;
  /** Answers whether the poll was sent; a false keeps the dialog open. */
  onCreatePoll?: (poll: WaGroupPollRequest) => Promise<boolean>;
  creatingPoll?: boolean;
}

function AttachMenu({
  attaching,
  onFile,
  onPoll,
}: {
  attaching: boolean;
  onFile: (file: File) => void;
  onPoll?: () => void;
}) {
  // false: antd would otherwise post the file itself, without the session
  // cookie or the caption.
  const upload = (
    <Upload
      accept={CS_MEDIA_ACCEPT}
      showUploadList={false}
      beforeUpload={(file) => {
        onFile(file);
        return false;
      }}
    >
      Foto atau berkas
    </Upload>
  );
  const items = [
    { key: "file", icon: <PaperClipOutlined />, label: upload },
    ...(onPoll
      ? [{ key: "poll", icon: <BarChartOutlined />, label: "Buat polling" }]
      : []),
  ];
  return (
    <Dropdown
      trigger={["click"]}
      placement="topLeft"
      menu={{ items, onClick: ({ key }) => key === "poll" && onPoll?.() }}
    >
      <Button
        type="text"
        icon={<PaperClipOutlined />}
        loading={attaching}
        title="Lampirkan"
        aria-label="Lampirkan"
      />
    </Dropdown>
  );
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
  participants,
  onSend,
  onAttach,
  sending,
  attaching,
  replyTo,
  onCancelReply,
  onTypingChange = () => {},
  onCreatePoll,
  creatingPoll = false,
}: GroupComposerProps) {
  const [text, setText] = useState("");
  const [pollOpen, setPollOpen] = useState(false);
  const signalTyping = useTypingSignal(group.id, onTypingChange);
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
    const typed = text.trim();
    if (!staged && !typed) return;
    signalTyping(false);
    const { body, mentions } = toWhatsAppMentions(typed, participants);
    const sent = staged
      ? await onAttach(staged, body, mentions)
      : await onSend(body, mentions);
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

  const createPoll = async (poll: WaGroupPollRequest) => {
    if (await onCreatePoll?.(poll)) setPollOpen(false);
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
        <AttachMenu
          attaching={attaching}
          onFile={stage}
          onPoll={onCreatePoll ? () => setPollOpen(true) : undefined}
        />
        {/* onPressEnter only fires while no member list is open, so Enter
            picks a member there and sends everywhere else. */}
        <Mentions
          value={text}
          onChange={(value) => {
            setText(value);
            signalTyping(value.trim().length > 0);
          }}
          onBlur={() => signalTyping(false)}
          options={mentionOptions(participants)}
          onPaste={handlePaste}
          onPressEnter={(e) => {
            if (!e.shiftKey) {
              e.preventDefault();
              handleSend();
            }
          }}
          autoSize={{ minRows: 1, maxRows: 5 }}
          placement="top"
          placeholder="Tulis pesan ke grup, ketik @ untuk menyebut anggota"
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
      <CreatePollModal
        open={pollOpen}
        creating={creatingPoll}
        onCreate={createPoll}
        onClose={() => setPollOpen(false)}
      />
    </div>
  );
}
