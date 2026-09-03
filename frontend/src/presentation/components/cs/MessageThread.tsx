import { useState } from "react";
import { Alert, Button, Image, Modal, Typography } from "antd";
import {
  CheckOutlined,
  ClockCircleOutlined,
  FileOutlined,
  PlayCircleFilled,
  RedoOutlined,
} from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface MessageThreadProps {
  messages: CsMessage[];
  onRetry: (body: string) => void;
}

function clock(iso: string): string {
  return new Date(iso).toLocaleTimeString("id-ID", {
    hour: "2-digit",
    minute: "2-digit",
  });
}

/**
 * How far a reply got, in the shorthand a CS already reads on their phone: one
 * tick left the app, two arrived, two in colour were read. A queued reply shows
 * a clock, because "sent" would be a lie while it is still waiting.
 */
function DeliveryMark({ status }: { status: CsMessage["status"] }) {
  const marks: Partial<
    Record<CsMessage["status"], { icon: JSX.Element; label: string }>
  > = {
    queued: {
      icon: <ClockCircleOutlined style={{ fontSize: 11 }} />,
      label: "Menunggu dikirim",
    },
    sent: {
      icon: <CheckOutlined style={{ fontSize: 11 }} />,
      label: "Terkirim",
    },
    delivered: {
      icon: (
        <span style={{ letterSpacing: -4 }}>
          <CheckOutlined style={{ fontSize: 11 }} />
          <CheckOutlined style={{ fontSize: 11 }} />
        </span>
      ),
      label: "Sampai di HP pelanggan",
    },
    read: {
      icon: (
        <span style={{ letterSpacing: -4, color: colors.success }}>
          <CheckOutlined style={{ fontSize: 11 }} />
          <CheckOutlined style={{ fontSize: 11 }} />
        </span>
      ),
      label: "Dibaca",
    },
  };

  const mark = marks[status];
  if (!mark) return null;
  return (
    <span aria-label={mark.label} title={mark.label}>
      {mark.icon}
    </span>
  );
}

/**
 * A video shows as a still with a play button, the way it does in any chat —
 * and opens on click. Played inline at its own size a portrait clip filled the
 * thread the way an uncapped photo once did, and a bare player gives no hint
 * that there is anything to watch.
 *
 * The still is the video's own first frame: preload="metadata" is enough for a
 * browser to draw it, so no poster has to be generated or stored anywhere.
 */
function VideoAttachment({
  src,
  spacing,
}: {
  src: string;
  spacing: React.CSSProperties;
}) {
  const [open, setOpen] = useState(false);

  return (
    <>
      <div
        role="button"
        tabIndex={0}
        aria-label="Putar video"
        onClick={() => setOpen(true)}
        onKeyDown={(e) => {
          if (e.key === "Enter" || e.key === " ") {
            e.preventDefault();
            setOpen(true);
          }
        }}
        style={{
          ...spacing,
          position: "relative",
          width: 220,
          height: 160,
          borderRadius: 6,
          overflow: "hidden",
          cursor: "pointer",
          background: "#000",
        }}
      >
        <video
          src={src}
          preload="metadata"
          muted
          style={{ width: "100%", height: "100%", objectFit: "cover" }}
        />
        <span
          style={{
            position: "absolute",
            inset: 0,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            color: "#fff",
            fontSize: 34,
            textShadow: "0 1px 6px rgba(0,0,0,0.6)",
          }}
        >
          <PlayCircleFilled />
        </span>
      </div>

      <Modal
        open={open}
        onCancel={() => setOpen(false)}
        footer={null}
        width={720}
        centered
        destroyOnClose
      >
        {/* autoPlay: the click that opened this was the request to watch it. */}
        <video
          src={src}
          controls
          autoPlay
          style={{ width: "100%", maxHeight: "70vh" }}
        />
      </Modal>
    </>
  );
}

/**
 * Everything that is not a photo or plain text. A customer sends a video of a
 * blinking modem far more often than they describe it, and this used to render
 * as the word "Lampiran" — no player, no link, nothing to open. The file was
 * already downloaded and already served; only the way to see it was missing.
 */
function MediaAttachment({ message }: { message: CsMessage }) {
  const src = `${env.apiUrl}${API_ENDPOINTS.CS_MEDIA(message.id)}`;
  const spacing = { display: "block", marginBottom: message.body ? 6 : 2 };

  if (message.kind === "video") {
    return <VideoAttachment src={src} spacing={spacing} />;
  }

  if (message.kind === "audio") {
    return (
      <audio
        controls
        preload="metadata"
        src={src}
        style={{ ...spacing, width: 260, maxWidth: "100%" }}
      />
    );
  }

  // A document has nothing to render in place, so it gets the one thing that
  // is useful: its name, and a way to open it.
  return (
    <a
      href={src}
      target="_blank"
      rel="noreferrer"
      style={{
        ...spacing,
        display: "flex",
        alignItems: "center",
        gap: 8,
        padding: "8px 10px",
        borderRadius: 6,
        background: "rgba(255, 255, 255, 0.04)",
        color: colors.textBody,
        fontSize: 13,
        wordBreak: "break-all",
      }}
    >
      <FileOutlined />
      {message.mediaFilename || "Buka lampiran"}
    </a>
  );
}

function MessageBubble({
  message,
  onRetry,
}: {
  message: CsMessage;
  onRetry: (body: string) => void;
}) {
  const outgoing = message.direction === "out";

  return (
    <div
      style={{
        display: "flex",
        justifyContent: outgoing ? "flex-end" : "flex-start",
        marginBottom: 6,
      }}
    >
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

        {message.body && (
          <Text
            style={{
              color: colors.textBody,
              fontSize: 14,
              whiteSpace: "pre-wrap",
              wordBreak: "break-word",
            }}
          >
            {message.body}
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
    </div>
  );
}

/** Draws a thread top-to-bottom, newest last, the way a chat reads. History
 * itself arrives newest first, so this is the one place that reverses it. */
export function MessageThread({ messages, onRetry }: MessageThreadProps) {
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
        <MessageBubble key={message.id} message={message} onRetry={onRetry} />
      ))}
    </div>
  );
}
