import type { MessageDirection, MessageKind, MessageStatus } from "./CsMessage";
import type {
  WaGroupMessageExtra,
  WaGroupReaction,
} from "./WaGroupMessageExtra";

/** A group message can be more than an inbox one: a notice WhatsApp writes
 * itself ("Budi menambahkan Rina"), or a shared place, contact or poll. */
export type WaGroupMessageKind =
  | MessageKind
  | "system"
  | "location"
  | "contact"
  | "poll";

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
  /** GET /groups/:id/avatar has a photo to serve. */
  hasAvatar: boolean;
  /** Pinned to the top of the list, for the whole team. */
  pinned: boolean;
  createdAt: string;
  updatedAt: string;
}

export type WaGroupRole = "member" | "admin" | "superadmin";

export interface WaGroupParticipant {
  jid: string;
  /** Digits, "" when WhatsApp only gave a LID. */
  phone: string;
  /** The address-book name, "" when unknown. */
  name: string;
  /** The name the member gave their own WhatsApp profile, "" if never seen. */
  pushName: string;
  hasAvatar: boolean;
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
  kind: WaGroupMessageKind;
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
  /** The sender's profile name when it was sent, "" when none. */
  senderPushName: string;
  senderHasAvatar: boolean;
  /** The TikMan user who wrote it; absent when it was typed on the phone. */
  senderUserId?: string;
  kind: WaGroupMessageKind;
  /** For a system message, the finished Indonesian sentence; for a place,
   * contact or poll, a plain summary of what extra holds. */
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
  /** Display names tagged in the body as "@<name>". */
  mentionNames: string[];
  edited: boolean;
  /** Deleted for everyone: body and media are gone. */
  deleted: boolean;
  forwarded: boolean;
  /** Starred by anyone on the team. */
  starred: boolean;
  reactions: WaGroupReaction[];
  /** Out messages only. */
  readCount?: number;
  deliveredCount?: number;
  /** The other members when it was sent. */
  memberCount?: number;
  extra?: WaGroupMessageExtra;
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

/** Requests on a single message, or on the group's history. Unlike the
 * management actions any CS may make them. */
export type WaGroupMessageActionKind =
  | "react"
  | "edit"
  | "revoke"
  | "vote"
  | "history";

/**
 * A management request carried out by the WhatsApp process in the background.
 * It comes back queued; the final status arrives later over the stream.
 */
export interface WaGroupAction {
  id: string;
  waAccountId: string;
  /** Absent for a create until it is done. */
  groupId?: string;
  action: WaGroupActionKind | WaGroupMessageActionKind;
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
