import { useState } from "react";
import { Modal } from "antd";
import { FileOutlined, PlayCircleFilled } from "@ant-design/icons";
import type { CsMessage } from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";
import { colors } from "@/shared/theme/colors";

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
export function MediaAttachment({ message }: { message: CsMessage }) {
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
