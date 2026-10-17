import type { MessageDirection, MessageKind, MessageStatus } from "./CsMessage";

/**
 * One WhatsApp group a connected number belongs to, or once belonged to. Group
 * traffic is kept apart from the customer inbox on purpose: a busy group would
 * otherwise bury the private chats a CS has to answer.
 */
export interface WaGroup {
  id: string;
  waAccountId: string;
  jid: string;
  name: string;
  /** The group description, "" when it has none. */
  topic: string;
  ownerJid: string;
  participantCount: number;
  /** Our number is an admin (or the owner) of this group. Not the TikMan
   * user's role — that is a separate question. */
  isAdmin: boolean;
  /** Only group admins may send. */
  announce: boolean;
  /** Only group admins may edit the name and description. */
  locked: boolean;
  /** Our number is no longer a member; the history stays readable. */
  left: boolean;
  lastMessageAt?: string;
  lastMessagePreview: string;
  /** "" when nothing has been said in the group yet. */
  lastMessageKind: MessageKind | "";
  lastMessageSender: string;
  /** Who spoke last: "out" is our own number, shown as "Anda". */
  lastMessageDirection?: "in" | "out" | "";
  unreadCount: number;
  createdAt: string;
  updatedAt: string;
}

export type WaGroupRole = "member" | "admin" | "superadmin";

export interface WaGroupParticipant {
  jid: string;
  /** Digits, "" when WhatsApp only gave a LID. */
  phone: string;
  name: string;
  role: WaGroupRole;
  isSelf: boolean;
}

export interface WaGroupDetail {
  group: WaGroup;
  participants: WaGroupParticipant[];
}

/** As much of a quoted group message as the block above a reply needs. */
export interface WaGroupQuotedMessage {
  id: string;
  direction: MessageDirection;
  kind: MessageKind;
  body: string;
  mediaFilename?: string;
  senderName: string;
}

export interface WaGroupMessage {
  id: string;
  groupId: string;
  waMessageId?: string;
  /** "out" is anything our number sent, from TikMan or from the phone. */
  direction: MessageDirection;
  senderJid: string;
  senderPhone: string;
  senderName: string;
  /** The TikMan user who wrote it; absent when it was typed on the phone. */
  senderUserId?: string;
  kind: MessageKind;
  body: string;
  mediaMime?: string;
  mediaSize?: number;
  mediaFilename?: string;
  status: MessageStatus;
  failReason?: string;
  replyToId?: string;
  replyTo?: WaGroupQuotedMessage;
  previewUrl?: string;
  previewTitle?: string;
  previewDescription?: string;
  /** base64 JPEG. */
  previewThumbnail?: string;
  waTimestamp: string;
  createdAt: string;
}

export type WaGroupActionKind =
  | "create"
  | "rename"
  | "set_topic"
  | "add"
  | "remove"
  | "promote"
  | "demote"
  | "set_announce"
  | "set_locked"
  | "invite_link"
  | "reset_invite_link"
  | "leave";

/**
 * A management request carried out by the WhatsApp process in the background.
 * It comes back queued; the final status arrives later over the stream.
 */
export interface WaGroupAction {
  id: string;
  waAccountId: string;
  /** Absent for a create until it is done. */
  groupId?: string;
  action: WaGroupActionKind;
  status: "queued" | "done" | "failed";
  /** The invite link for invite_link/reset_invite_link; per-participant
   * problems for the member actions; "" otherwise. */
  result: string;
  failReason: string;
  createdAt: string;
  doneAt?: string;
}

/** The body of POST /groups/:id/actions. Which fields an action needs is the
 * server's rule; this only names them. */
export interface WaGroupActionRequest {
  action: Exclude<WaGroupActionKind, "create">;
  name?: string;
  topic?: string;
  participants?: string[];
  enabled?: boolean;
}

export interface WaGroupFilter {
  waAccountId?: string;
  search?: string;
}
