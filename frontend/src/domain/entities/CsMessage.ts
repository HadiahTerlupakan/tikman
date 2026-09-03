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
