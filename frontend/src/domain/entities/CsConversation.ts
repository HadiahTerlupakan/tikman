export type ConversationStatus = "unassigned" | "open" | "closed";

/** The line the inbox list shows under a customer's name. */
export interface CsLastMessage {
  body: string;
  kind: "text" | "image" | "document" | "audio" | "video";
  direction: "in" | "out";
  at: string;
}

/** One customer's WhatsApp thread on one connected number. */
export interface CsConversation {
  id: string;
  customerPhone: string;
  customerName: string;
  assignedUserId?: string;
  status: ConversationStatus;
  ontId?: string;
  lastMessageAt: string;
  unreadCount: number;
  /** Whether this customer has a profile photo stored. Most do not — a photo
   * is usually hidden from anyone outside the owner's contacts — so this is
   * what keeps the list from pointing every avatar at an endpoint that would
   * 404 on most rows, on every refresh. */
  hasAvatar: boolean;
  /** Which side spoke last. "in" is a thread still waiting on a CS. */
  lastMessageDirection?: "in" | "out";
  /** Absent on a thread nothing has been said in yet. */
  lastMessage?: CsLastMessage;
}

/**
 * The inbox's views — mine, awaiting a reply, closed — are mutually exclusive
 * on the backend: ListConversations checks them in that order and the first
 * one set wins. Setting more than one here is meaningless, not additive.
 */
export interface CsConversationFilter {
  search?: string;
  limit?: number;
  offset?: number;
  mine?: boolean;
  /** Every thread whose last message came from the customer, whoever holds
   * it — one rule covering both the chat nobody has answered and the customer
   * who wrote again after theirs was closed. */
  awaitingReply?: boolean;
  closed?: boolean;
}
