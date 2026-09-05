import { Button, Typography } from "antd";
import { CloseOutlined } from "@ant-design/icons";
import type { CsLinkPreview } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

interface LinkPreviewCardProps {
  preview: CsLinkPreview;
  /** Absent in a sent message: the card is part of what the customer already
   * received, so there is nothing left to decide. */
  onDismiss?: () => void;
}

/**
 * What the customer will see above the message, shown to the CS before they
 * send it.
 *
 * The green edge is WhatsApp's own idiom for this card, and it is the reason
 * the card reads as part of the message rather than as a control of ours.
 */
export function LinkPreviewCard({ preview, onDismiss }: LinkPreviewCardProps) {
  let host = preview.url;
  try {
    host = new URL(preview.url).host;
  } catch {
    // A URL the server accepted but the browser cannot parse is not worth a
    // broken card — fall back to showing it whole.
  }

  return (
    <div
      style={{
        display: "flex",
        alignItems: "flex-start",
        gap: 10,
        margin: "0 0 8px",
        padding: 8,
        background: colors.surface,
        borderLeft: `3px solid ${colors.success}`,
        borderRadius: 6,
      }}
    >
      {preview.thumbnail && (
        <img
          src={`data:image/jpeg;base64,${preview.thumbnail}`}
          alt=""
          style={{
            width: 56,
            height: 56,
            objectFit: "cover",
            borderRadius: 4,
            flexShrink: 0,
          }}
        />
      )}

      <div style={{ flex: 1, minWidth: 0 }}>
        <Text
          strong
          style={{
            color: colors.textPrimary,
            fontSize: 13,
            display: "block",
            overflow: "hidden",
            textOverflow: "ellipsis",
            whiteSpace: "nowrap",
          }}
        >
          {preview.title}
        </Text>
        {preview.description && (
          <Text
            style={{
              color: colors.textMuted,
              fontSize: 12,
              display: "-webkit-box",
              WebkitLineClamp: 2,
              WebkitBoxOrient: "vertical",
              overflow: "hidden",
            }}
          >
            {preview.description}
          </Text>
        )}
        <Text
          style={{ color: colors.textMuted, fontSize: 11, display: "block" }}
        >
          {host}
        </Text>
      </div>

      {onDismiss && (
        <Button
          type="text"
          size="small"
          icon={<CloseOutlined />}
          onClick={onDismiss}
          aria-label="Sembunyikan pratinjau tautan"
          title="Sembunyikan pratinjau"
        />
      )}
    </div>
  );
}
