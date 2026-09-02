import type {
  CsConversation,
  CsConversationFilter,
  CsMessage,
  CsQuickReply,
} from "../entities";

export interface ICsRepository {
  getConversations(filter?: CsConversationFilter): Promise<CsConversation[]>;
  getHistory(
    conversationId: string,
    limit?: number,
    offset?: number,
  ): Promise<CsMessage[]>;
  sendMessage(conversationId: string, body: string): Promise<CsMessage>;
  assign(conversationId: string, userId: string): Promise<CsConversation>;
  setStatus(conversationId: string, status: "closed"): Promise<CsConversation>;
  linkOnt(
    conversationId: string,
    ontId: string | null,
  ): Promise<CsConversation>;
  getQuickReplies(): Promise<CsQuickReply[]>;
}
