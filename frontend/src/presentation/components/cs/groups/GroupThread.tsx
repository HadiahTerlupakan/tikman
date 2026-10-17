import { useState } from "react";
import { Alert, Button, Typography } from "antd";
import { RedoOutlined } from "@ant-design/icons";
import type { WaGroupMessage } from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import { colors } from "@/shared/theme/colors";
import { BubbleActions } from "../BubbleActions";
import { DeliveryMark } from "../DeliveryMark";
import { LinkPreviewCard } from "../LinkPreviewCard";
import { ImageAttachment, MediaAttachment } from "../MessageAttachment";
import { MessageText } from "../MessageText";
import { QuotedBlock } from "../QuotedBlock";
import { senderLabel } from "./groupSummary";

const { Text } = Typography;

interface GroupThreadProps {
  /** Newest first, as the API returns them. */
  messages: WaGroupMessage[];
  onRetry: (body: string) => void;
  /** Absent where nothing can be sent, so the gesture is not offered only to
   * be refused. */
  onReply?: (message: WaGroupMessage) => void;
}

function anchor(messageId: string): string {
  return `wa-group-message-${messageId}`;
}

function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

/** Unlike a customer thread, a group has many people on the other side, so a
 * bubble names its sender — once per run, the way WhatsApp does, rather than
 * repeating the same name on every line somebody sends in a row. */
function startsRun(message: WaGroupMessage, previous?: WaGroupMessage) {
  if (message.direction === "out") return false;
  return !previous || previous.senderJid !== message.senderJid;
}

function quoteAuthor(quoted: NonNullable<WaGroupMessage["replyTo"]>) {
  if (quoted.direction === "out") return "Anda";
  return quoted.senderName || "Anggota";
}

function BubbleBody({ message }: { message: WaGroupMessage }) {
  const src = `${env.apiUrl}${API_ENDPOINTS.CS_GROUP_MEDIA(message.id)}`;
  const jump = message.replyTo
    ? () =>
        document
          .getElementById(anchor(message.replyTo!.id))
          ?.scrollIntoView({ behavior: "smooth", block: "center" })
    : undefined;

  return (
    <>
      {message.replyTo && (
        <QuotedBlock
          quoted={message.replyTo}
          authorLabel={quoteAuthor(message.replyTo)}
          onJump={jump}
        />
      )}
      {message.kind === "image" && (
        <ImageAttachment src={src} message={message} />
      )}
      {message.kind !== "image" && message.kind !== "text" && (
        <MediaAttachment src={src} message={message} />
      )}
      {message.previewTitle && (
        <LinkPreviewCard
          preview={{
            url: message.previewUrl ?? "",
            title: message.previewTitle,
            description: message.previewDescription,
            thumbnail: message.previewThumbnail,
          }}
        />
      )}
      {message.body && (
        <Text
          style={{
            color: colors.textBody,
            fontSize: 14,
            whiteSpace: "pre-wrap",
            wordBreak: "break-word",
          }}
        >
          <MessageText body={message.body} />
        </Text>
      )}
    </>
  );
}

function BubbleFooter({
  message,
  onRetry,
}: {
  message: WaGroupMessage;
  onRetry: (body: string) => void;
}) {
  const outgoing = message.direction === "out";
  return (
    <>
      <div
        style={{
          display: "flex",
          justifyContent: "flex-end",
          alignItems: "center",
          gap: 4,
          marginTop: 2,
          color: colors.textMuted,
          fontSize: 11,
        }}
      >
        {outgoing && !message.senderUserId && <span>dari HP</span>}
        <span>{clock(message.waTimestamp)}</span>
        {outgoing && <DeliveryMark status={message.status} />}
      </div>
      {message.status === "failed" && (
        <Alert
          type="error"
          showIcon
          message={message.failReason || "Gagal terkirim"}
          action={
            message.kind === "text" && (
              <Button
                size="small"
                icon={<RedoOutlined />}
                onClick={() => onRetry(message.body)}
              >
                Coba lagi
              </Button>
            )
          }
          style={{ marginTop: 6 }}
        />
      )}
    </>
  );
}

function GroupBubble({
  message,
  showSender,
  onRetry,
  onReply,
}: {
  message: WaGroupMessage;
  showSender: boolean;
  onRetry: (body: string) => void;
  onReply?: (message: WaGroupMessage) => void;
}) {
  const outgoing = message.direction === "out";
  const [hovered, setHovered] = useState(false);
  const actions = (
    <BubbleActions message={message} visible={hovered} onReply={onReply} />
  );

  return (
    <div
      id={anchor(message.id)}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 4,
        justifyContent: outgoing ? "flex-end" : "flex-start",
        marginBottom: 6,
        marginTop: showSender ? 8 : 0,
      }}
    >
      {outgoing && actions}
      <div
        style={{
          maxWidth: "68%",
          minWidth: 96,
          padding: "7px 10px 5px",
          background: outgoing ? "rgba(62, 207, 142, 0.14)" : "#27272a",
          borderRadius: 10,
          borderBottomRightRadius: outgoing ? 2 : 10,
          borderBottomLeftRadius: outgoing ? 10 : 2,
        }}
      >
        {showSender && (
          <Text
            strong
            style={{ display: "block", fontSize: 12, color: colors.warning }}
          >
            {senderLabel(message)}
          </Text>
        )}
        <BubbleBody message={message} />
        <BubbleFooter message={message} onRetry={onRetry} />
      </div>
      {!outgoing && actions}
    </div>
  );
}

/** A group's messages top to bottom, newest last. */
export function GroupThread({ messages, onRetry, onReply }: GroupThreadProps) {
  if (messages.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "48px 24px" }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          Belum ada pesan di grup ini.
        </Text>
      </div>
    );
  }

  const ordered = [...messages].reverse();
  return (
    <div style={{ width: "100%" }}>
      {ordered.map((message, i) => (
        <GroupBubble
          key={message.id}
          message={message}
          showSender={startsRun(message, ordered[i - 1])}
          onRetry={onRetry}
          onReply={onReply}
        />
      ))}
    </div>
  );
}
