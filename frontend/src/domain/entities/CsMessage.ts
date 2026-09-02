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
  waTimestamp: string;
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
