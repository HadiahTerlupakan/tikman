/** How much a paired number may do on a WhatsApp Channel. Only the roles that
 * may post are ever stored, so the picker never offers one that would refuse
 * the update. */
export type ChannelRole = "owner" | "admin";

/** How far an update has travelled. There is no "delivered" or "read": a
 * channel sends no receipts, so promising either on screen would be a lie. */
export type BroadcastPostStatus = "queued" | "sent" | "failed";

/** Where one announcement goes. A channel names its channel; a status names
 * nothing beyond the number it goes out from. */
export type BroadcastDestination = "channel" | "status";

/** One destination as the composer asks for it. */
export type BroadcastTarget =
  | { type: "channel"; channelId: string }
  | { type: "status"; waAccountId: string };

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

/** One announcement, on its way to a channel or status, or already gone. */
export interface BroadcastPost {
  id: string;
  waAccountId: string;
  destination: BroadcastDestination;
  destinationJid?: string;
  senderUserId: string;
  kind: string;
  body: string;
  mediaFilename?: string;
  status: BroadcastPostStatus;
  failReason?: string;
  createdAt: string;
  sentAt?: string;
}
