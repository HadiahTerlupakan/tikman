/**
 * What the CS media endpoint accepts. This mirrors two things the server
 * enforces on its own — `mediaExtensions` in backend/internal/wa/media.go and
 * `maxUploadBytes` in backend/internal/api/cs_handler_media.go — and enforces
 * nothing itself. Its only job is to tell a CS that a file will be refused
 * before they spend a round trip finding out. A type or a limit changed on the
 * server has to be changed here too.
 */
export const CS_MEDIA_MIME_TYPES = [
  "image/jpeg",
  "image/png",
  "image/webp",
  "image/gif",
  "video/mp4",
  "video/3gpp",
  "audio/ogg",
  "audio/mpeg",
  "audio/mp4",
  "audio/amr",
  "application/pdf",
];

/** The `accept` attribute for a file input, from the list above. */
export const CS_MEDIA_ACCEPT = CS_MEDIA_MIME_TYPES.join(",");

export const CS_MEDIA_MAX_BYTES = 16 * 1024 * 1024;

/**
 * Answers why an attachment would be refused, or null when it would not.
 * The wording is the CS's, not the server's: they picked a file, and a
 * silent no-op or an English error code tells them nothing about which one.
 */
export function attachmentRejection(file: File): string | null {
  if (!CS_MEDIA_MIME_TYPES.includes(file.type)) {
    return `Jenis berkas ${file.type || "ini"} tidak bisa dikirim lewat WhatsApp`;
  }
  if (file.size > CS_MEDIA_MAX_BYTES) {
    return `Lampiran melebihi batas ${CS_MEDIA_MAX_BYTES / (1024 * 1024)} MB`;
  }
  return null;
}
