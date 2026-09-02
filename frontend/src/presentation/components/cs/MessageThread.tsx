import { Alert, Button, Space, Typography } from "antd";
import { RedoOutlined } from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";

const { Text } = Typography;

interface MessageThreadProps {
  messages: CsMessage[];
  onRetry: (body: string) => void;
}

// Direction decides which side of the thread a bubble sits on — "in" is the
// customer, "out" is the team, the same convention WhatsApp itself uses.
function bubbleAlign(direction: CsMessage["direction"]) {
  return direction === "out" ? "flex-end" : "flex-start";
}

function MessageBubble({
  message,
  onRetry,
}: {
  message: CsMessage;
  onRetry: (body: string) => void;
}) {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: bubbleAlign(message.direction),
      }}
    >
      <div
        style={{
          maxWidth: "70%",
          padding: 8,
          borderRadius: 8,
          background:
            message.direction === "out"
              ? "rgba(62, 207, 142, 0.12)"
              : "#27272a",
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
              borderRadius: 4,
              marginBottom: message.body ? 4 : 0,
            }}
          />
        )}
        {message.body && <Text>{message.body}</Text>}
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
            style={{ marginTop: 8 }}
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
    return <Text type="secondary">Belum ada pesan.</Text>;
  }

  const ordered = [...messages].reverse();

  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {ordered.map((message) => (
        <MessageBubble key={message.id} message={message} onRetry={onRetry} />
      ))}
    </Space>
  );
}
