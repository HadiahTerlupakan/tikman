import { Typography } from "antd";
import { StopOutlined } from "@ant-design/icons";
import type {
  MessageKind,
  WaGroupMessage,
  WaGroupMessageKind,
} from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import { colors } from "@/shared/theme/colors";
import { LinkPreviewCard } from "../LinkPreviewCard";
import { ImageAttachment, MediaAttachment } from "../MessageAttachment";
import { MessageText } from "../MessageText";
import { QuotedBlock } from "../QuotedBlock";
import { splitMentions } from "./groupMentions";
import { messageAnchor } from "./groupThreadLayout";
import { PollBubble } from "./PollBubble";
import { ContactCards, LocationCard } from "./SharedContentCards";

const { Text } = Typography;

const MEDIA_KINDS = new Set<WaGroupMessageKind>([
  "image",
  "document",
  "audio",
  "video",
  "sticker",
]);

function isMedia(kind: WaGroupMessageKind): kind is MessageKind {
  return MEDIA_KINDS.has(kind);
}

function quoteAuthor(quoted: NonNullable<WaGroupMessage["replyTo"]>) {
  if (quoted.direction === "out") return "Anda";
  return quoted.senderName || "Anggota";
}

/** The body with WhatsApp's mentions picked out, and links clickable in the
 * runs between them. */
function MentionText({ message }: { message: WaGroupMessage }) {
  return (
    <Text
      style={{
        color: colors.textBody,
        fontSize: 14,
        whiteSpace: "pre-wrap",
        wordBreak: "break-word",
      }}
    >
      {splitMentions(message.body, message.mentionNames ?? []).map((part, i) =>
        part.mention ? (
          <span key={i} style={{ color: colors.success, fontWeight: 500 }}>
            {part.text}
          </span>
        ) : (
          <MessageText key={i} body={part.text} />
        ),
      )}
    </Text>
  );
}

function Deleted() {
  return (
    <Text italic style={{ color: colors.textMuted, fontSize: 13 }}>
      <StopOutlined style={{ marginRight: 6 }} />
      Pesan ini telah dihapus
    </Text>
  );
}

/** What a place, contact or poll draws instead of its plain-text summary.
 * Answers null for every other kind, and for one that came without the
 * structured part, which then falls back to the summary in body. */
function sharedContent(
  message: WaGroupMessage,
  onVote?: (options: string[]) => void,
) {
  const extra = message.extra;
  if (!extra) return null;
  if (message.kind === "location") return <LocationCard extra={extra} />;
  if (message.kind === "contact") return <ContactCards extra={extra} />;
  if (message.kind === "poll" && extra.options) {
    return (
      <PollBubble
        poll={{
          question: extra.question ?? "",
          selectableCount: extra.selectableCount ?? 0,
          options: extra.options,
        }}
        onVote={onVote}
      />
    );
  }
  return null;
}

export function GroupBubbleBody({
  message,
  onVote,
}: {
  message: WaGroupMessage;
  onVote?: (options: string[]) => void;
}) {
  if (message.deleted) return <Deleted />;
  const src = `${env.apiUrl}${API_ENDPOINTS.CS_GROUP_MEDIA(message.id)}`;
  const quoted = message.replyTo;
  const jump = quoted
    ? () =>
        document
          .getElementById(messageAnchor(quoted.id))
          ?.scrollIntoView({ behavior: "smooth", block: "center" })
    : undefined;
  const shared = sharedContent(message, onVote);
  const kind = message.kind;

  return (
    <>
      {message.forwarded && (
        <Text
          italic
          style={{ display: "block", fontSize: 11, color: colors.textMuted }}
        >
          ↪ Diteruskan
        </Text>
      )}
      {quoted && (
        <QuotedBlock
          quoted={quoted}
          authorLabel={quoteAuthor(quoted)}
          onJump={jump}
        />
      )}
      {kind === "image" && (
        <ImageAttachment src={src} message={{ ...message, kind }} />
      )}
      {isMedia(kind) && kind !== "image" && (
        <MediaAttachment src={src} message={{ ...message, kind }} />
      )}
      {shared}
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
      {!shared && message.body && <MentionText message={message} />}
    </>
  );
}
