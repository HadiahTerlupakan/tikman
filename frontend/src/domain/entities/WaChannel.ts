/** How much a paired number may do on a WhatsApp Channel. Only the roles that
 * may post are ever stored, so the picker never offers one that would refuse
 * the update. */
export type ChannelRole = "owner" | "admin";

/** How far an update has travelled. There is no "delivered" or "read": a
 * channel sends no receipts, so promising either on screen would be a lie. */
export type ChannelPostStatus = "queued" | "sent" | "failed";

/** One WhatsApp Channel a paired number administers. */
export interface WaChannel {
  id: string;
  waAccountId: string;
  jid: string;
  name: string;
  role: ChannelRole;
  subscriberCount: number;
  syncedAt: string;
}

/** One update, on its way to a channel or already gone. */
export interface ChannelPost {
  id: string;
  waAccountId: string;
  channelJid: string;
  senderUserId: string;
  kind: string;
  body: string;
  mediaFilename?: string;
  status: ChannelPostStatus;
  failReason?: string;
  createdAt: string;
  sentAt?: string;
}
