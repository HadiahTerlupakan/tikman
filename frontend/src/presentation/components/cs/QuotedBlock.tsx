import { Typography } from "antd";
import {
  AudioOutlined,
  FileOutlined,
  PictureOutlined,
  VideoCameraOutlined,
} from "@ant-design/icons";
import type { CsQuotedMessage } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";

const { Text } = Typography;

const attachmentLabels: Record<string, { icon: JSX.Element; noun: string }> = {
  image: { icon: <PictureOutlined />, noun: "Foto" },
  video: { icon: <VideoCameraOutlined />, noun: "Video" },
  audio: { icon: <AudioOutlined />, noun: "Pesan suara" },
  document: { icon: <FileOutlined />, noun: "Dokumen" },
};

/** What the quoted line reads when the message being quoted is not text. A
 * photo with no caption has no words of its own, and an empty line under the
 * author's name looks like a rendering fault rather than a picture. */
function attachmentLine(quoted: CsQuotedMessage) {
  const label = attachmentLabels[quoted.kind];
  if (!label) return null;
  return (
    <>
      {label.icon} {quoted.mediaFilename || label.noun}
    </>
  );
}

interface QuotedBlockProps {
  quoted: CsQuotedMessage;
  /** Who wrote the quoted message, in this inbox's own words. */
  authorLabel: string;
  /** Jumps to the quoted message. Omitted in the composer, where there is
   * nothing above to jump to yet. */
  onJump?: () => void;
}

/**
 * The block above a reply naming what it answers. One colour bar, one author,
 * one line of what was said — the same shape WhatsApp draws, and the same
 * shape in the composer while the reply is still being written, so a CS sees
 * before sending exactly what the customer will see after.
 */
export function QuotedBlock({ quoted, authorLabel, onJump }: QuotedBlockProps) {
  const fromCustomer = quoted.direction === "in";
  const line = quoted.body || attachmentLine(quoted);

  return (
    <div
      role={onJump ? "button" : undefined}
      tabIndex={onJump ? 0 : undefined}
      aria-label={onJump ? `Lihat pesan dari ${authorLabel}` : undefined}
      onClick={onJump}
      onKeyDown={(e) => {
        if (onJump && (e.key === "Enter" || e.key === " ")) {
          e.preventDefault();
          onJump();
        }
      }}
      style={{
        display: "flex",
        flexDirection: "column",
        gap: 1,
        marginBottom: 4,
        padding: "4px 8px",
        // The bar carries the author, so the two sides of a conversation stay
        // apart at a glance even when the words alone would not say which.
        // Our own replies take the accent: a CS scanning a thread is looking
        // for which answer a customer came back on.
        borderLeft: `3px solid ${fromCustomer ? colors.textSecondary : colors.success}`,
        borderRadius: 4,
        background: "rgba(255, 255, 255, 0.06)",
        cursor: onJump ? "pointer" : "default",
      }}
    >
      <Text
        style={{
          fontSize: 12,
          fontWeight: 600,
          color: fromCustomer ? colors.textSecondary : colors.success,
        }}
      >
        {authorLabel}
      </Text>
      <Text
        ellipsis
        style={{ fontSize: 12, color: colors.textMuted, maxWidth: 320 }}
      >
        {line}
      </Text>
    </div>
  );
}
