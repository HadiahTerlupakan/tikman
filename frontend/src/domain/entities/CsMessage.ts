export type MessageDirection = "in" | "out";
export type MessageKind = "text" | "image" | "document" | "audio" | "video";
export type MessageStatus = "queued" | "sent" | "delivered" | "read" | "failed";

/** One WhatsApp message in a thread. */
export interface CsMessage {
  id: string;
  conversationId: string;
  direction: MessageDirection;
  senderUserId?: string;
  kind: MessageKind;
  body: string;
  mediaMime?: string;
  mediaFilename?: string;
  status: MessageStatus;
  failReason?: string;
  replyToId?: string;
  /** The message this one quotes, resolved by the API. Absent when it quotes
   * nothing, and also when what it quoted has since been swept by retention —
   * the reply outlives the message it answered. */
  replyTo?: CsQuotedMessage;
  /** The link card stored with the message. Present on either direction: an
   * outgoing one is what we fetched to attach, an incoming one is what the
   * customer's own WhatsApp sent. */
  previewUrl?: string;
  previewTitle?: string;
  previewDescription?: string;
  /** base64 JPEG. */
  previewThumbnail?: string;
  waTimestamp: string;
}

/** As much of a quoted message as the grey block above a reply needs. */
export interface CsQuotedMessage {
  id: string;
  direction: MessageDirection;
  kind: MessageKind;
  body: string;
  mediaFilename?: string;
}

/** A canned reply a CS can insert instead of retyping it. */
export interface CsQuickReply {
  id: string;
  title: string;
  body: string;
  createdBy: string;
  createdAt: string;
  updatedAt: string;
}

/** What a link in a draft resolves to, for the card the composer draws.
 * Display only — the send path resolves the page again for itself. */
export interface CsLinkPreview {
  url: string;
  title: string;
  description?: string;
  /** base64 JPEG, absent when the page named no usable image. */
  thumbnail?: string;
}
