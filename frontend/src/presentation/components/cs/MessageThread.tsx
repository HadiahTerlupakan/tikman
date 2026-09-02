import { Alert, Button, Typography } from "antd";
import {
  CheckOutlined,
  ClockCircleOutlined,
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
          // Content-Disposition on this endpoint is "attachment", but that
          // only steers a direct navigation to save the file — it does not
          // stop a subresource fetch like this from rendering inline.
          <img
            src={`${env.apiUrl}${API_ENDPOINTS.CS_MEDIA(message.id)}`}
            alt={message.mediaFilename || "lampiran"}
            style={{
              maxWidth: "100%",
              borderRadius: 6,
              marginBottom: message.body ? 6 : 2,
              display: "block",
            }}
          />
        )}

        {message.kind !== "image" && message.kind !== "text" && (
          <Text style={{ color: colors.textSecondary, fontSize: 13 }}>
            {message.mediaFilename || "Lampiran"}
          </Text>
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
