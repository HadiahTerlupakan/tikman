import { useState } from "react";
import { Alert, Button, Image, Typography } from "antd";
import { RedoOutlined } from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";
import { BubbleActions } from "./BubbleActions";
import { DeliveryMark } from "./DeliveryMark";
import { MediaAttachment } from "./MessageAttachment";
import { QuotedBlock } from "./QuotedBlock";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import { colors } from "@/shared/theme/colors";
import { MessageText } from "./MessageText";
import { LinkPreviewCard } from "./LinkPreviewCard";

const { Text } = Typography;

interface MessageThreadProps {
  messages: CsMessage[];
  onRetry: (body: string) => void;
  /** Starts a reply quoting this message. Absent for anyone who cannot send
   * on this thread — offering the gesture and then refusing the send would be
   * the worse of the two. */
  onReply?: (message: CsMessage) => void;
  /** Removes this message from the inbox. Absent for the same reason as
   * onReply, and on the same terms: it removes the copy held here, never the
   * one on the customer's phone. */
  onDelete?: (message: CsMessage) => void;
}

/** The name a quote puts above what it quotes. The customer's own name is not
 * on a message row, so their side is named by their side of the conversation
 * rather than guessed at. */
function quoteAuthor(direction: CsMessage["direction"]): string {
  return direction === "out" ? "Anda" : "Pelanggan";
}

/** Where a quoted message lives in the page, so clicking a quote can jump to
 * it. Ids come from the API, so they are unique across the thread. */
function bubbleAnchor(messageId: string): string {
  return `cs-message-${messageId}`;
}

function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

function MessageBubble({
  message,
  onRetry,
  onReply,
  onDelete,
}: {
  message: CsMessage;
  onRetry: (body: string) => void;
  onReply?: (message: CsMessage) => void;
  onDelete?: (message: CsMessage) => void;
}) {
  const outgoing = message.direction === "out";
  const [hovered, setHovered] = useState(false);

  // A quoted message that is on the page can be jumped to. One swept by
  // retention, or simply older than what is loaded, still draws its block —
  // it just does not offer a jump that would go nowhere.
  const jumpToQuoted = message.replyTo
    ? () => {
        document
          .getElementById(bubbleAnchor(message.replyTo!.id))
          ?.scrollIntoView({ behavior: "smooth", block: "center" });
      }
    : undefined;

  return (
    <div
      id={bubbleAnchor(message.id)}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
      style={{
        display: "flex",
        alignItems: "center",
        gap: 4,
        justifyContent: outgoing ? "flex-end" : "flex-start",
        marginBottom: 6,
      }}
    >
      {outgoing && (
        <BubbleActions
          message={message}
          visible={hovered}
          onReply={onReply}
          onDelete={onDelete}
        />
      )}
      <div
        style={{
          maxWidth: "68%",
          minWidth: 96,
          padding: "7px 10px 5px",
          background: outgoing ? "rgba(62, 207, 142, 0.14)" : "#27272a",
          // One square corner on the speaker's side is the shape a chat has;
          // four equal corners read as a card, not a message.
          borderRadius: 10,
          borderBottomRightRadius: outgoing ? 2 : 10,
          borderBottomLeftRadius: outgoing ? 10 : 2,
        }}
      >
        {message.replyTo && (
          <QuotedBlock
            quoted={message.replyTo}
            authorLabel={quoteAuthor(message.replyTo.direction)}
            onJump={jumpToQuoted}
          />
        )}

        {message.kind === "image" && (
          // A thumbnail, not the photo. Customers send screenshots of whole
          // phone screens, and drawn at their natural height one of those
          // stretches the thread until the rest of the conversation is off
          // the page. Clicking opens it full size.
          //
          // Content-Disposition on this endpoint is "attachment", but that
          // only steers a direct navigation to save the file — it does not
          // stop a subresource fetch like this from rendering inline.
          <Image
            src={`${env.apiUrl}${API_ENDPOINTS.CS_MEDIA(message.id)}`}
            alt={message.mediaFilename || "lampiran"}
            width={220}
            height={160}
            preview={{ mask: "Lihat penuh" }}
            style={{
              objectFit: "cover",
              borderRadius: 6,
              display: "block",
            }}
            // Block, not antd's default inline-block: a caption is a sibling
            // span, and inline-block puts it beside the photo instead of
            // beneath it, running off the edge of the bubble.
            wrapperStyle={{
              display: "block",
              marginBottom: message.body ? 6 : 2,
            }}
          />
        )}

        {message.kind !== "image" && message.kind !== "text" && (
          <MediaAttachment message={message} />
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
          <span>{clock(message.waTimestamp)}</span>
          {outgoing && <DeliveryMark status={message.status} />}
        </div>

        {message.status === "failed" && (
          <Alert
            type="error"
            showIcon
            message={message.failReason || "Gagal terkirim"}
            action={
              <Button
                size="small"
                icon={<RedoOutlined />}
                onClick={() => onRetry(message.body)}
              >
                Coba lagi
              </Button>
            }
            style={{ marginTop: 6 }}
          />
        )}
      </div>
      {!outgoing && (
        <BubbleActions
          message={message}
          visible={hovered}
          onReply={onReply}
          onDelete={onDelete}
        />
      )}
    </div>
  );
}

/** Draws a thread top-to-bottom, newest last, the way a chat reads. History
 * itself arrives newest first, so this is the one place that reverses it. */
export function MessageThread({
  messages,
  onRetry,
  onReply,
  onDelete,
}: MessageThreadProps) {
  if (messages.length === 0) {
    return (
      <div style={{ textAlign: "center", padding: "48px 24px" }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          Belum ada pesan di percakapan ini.
        </Text>
      </div>
    );
  }

  const ordered = [...messages].reverse();

  return (
    <div style={{ width: "100%" }}>
      {ordered.map((message) => (
        <MessageBubble
          key={message.id}
          message={message}
          onRetry={onRetry}
          onReply={onReply}
          onDelete={onDelete}
        />
      ))}
    </div>
  );
}
