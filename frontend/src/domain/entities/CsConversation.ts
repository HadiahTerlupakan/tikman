export type ConversationStatus = "unassigned" | "open" | "closed";

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
}

/**
 * The inbox's views — mine, unassigned, closed — are mutually exclusive on
 * the backend: ListConversations checks them in that order and the first
 * one set wins. Setting more than one here is meaningless, not additive.
 */
export interface CsConversationFilter {
  search?: string;
  limit?: number;
  offset?: number;
  mine?: boolean;
  unassigned?: boolean;
  closed?: boolean;
}
