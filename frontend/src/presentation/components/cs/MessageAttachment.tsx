import { useState } from "react";
import { Image, Modal } from "antd";
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
        destroyOnHidden
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

/** What an attachment needs to know about the message carrying it — the
 * fields a group message shares with an inbox one. */
type AttachedMessage = Pick<
  CsMessage,
  "id" | "kind" | "body" | "mediaMime" | "mediaFilename"
>;

/**
 * A thumbnail, not the photo. Customers send screenshots of whole phone
 * screens, and drawn at their natural height one of those stretches the thread
 * until the rest of the conversation is off the page. Clicking opens it full
 * size.
 *
 * Content-Disposition on the media endpoints is "attachment", but that only
 * steers a direct navigation to save the file — it does not stop a subresource
 * fetch like this from rendering inline.
 */
export function ImageAttachment({
  src,
  message,
}: {
  src: string;
  message: AttachedMessage;
}) {
  return (
    <Image
      src={src}
      alt={message.mediaFilename || "lampiran"}
      width={220}
      height={160}
      preview={{ mask: "Lihat penuh" }}
      style={{ objectFit: "cover", borderRadius: 6, display: "block" }}
      // Block, not antd's default inline-block: a caption is a sibling span,
      // and inline-block puts it beside the photo instead of beneath it,
      // running off the edge of the bubble.
      wrapperStyle={{ display: "block", marginBottom: message.body ? 6 : 2 }}
    />
  );
}

/**
 * Everything that is not a photo or plain text. A customer sends a video of a
 * blinking modem far more often than they describe it, and this used to render
 * as the word "Lampiran" — no player, no link, nothing to open. The file was
 * already downloaded and already served; only the way to see it was missing.
 */
export function MediaAttachment({
  message,
  src = `${env.apiUrl}${API_ENDPOINTS.CS_MEDIA(message.id)}`,
}: {
  message: AttachedMessage;
  /** Where the file is served. Defaults to the inbox's media endpoint; group
   * messages are served from their own. */
  src?: string;
}) {
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

  // A sticker is square and carries its own transparency, so it takes neither
  // the photo's crop nor the document's link: contain, not cover, or the
  // artwork loses its edges.
  // An avatar sticker is a vector animation rather than a picture, so it falls
  // through to the file below: a broken-image icon reads as the inbox being
  // broken, not as a shape it cannot draw.
  if (message.kind === "sticker" && message.mediaMime?.startsWith("image/")) {
    return (
      <img
        src={src}
        alt="stiker"
        style={{ ...spacing, width: 140, height: 140, objectFit: "contain" }}
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
