import type {
  CsConversation,
  CsConversationFilter,
  CsMessage,
  CsQuickReply,
  WaAccount,
} from "../entities";

/** LinkONT also reports whether the customer's number reached the ONT row —
 * it does not when the phone is already recorded on a different ONT, and
 * that must not read the same as a plain success. */
export interface LinkOntResult {
  conversation: CsConversation;
  phoneRecorded: boolean;
}

export interface ICsRepository {
  getConversations(filter?: CsConversationFilter): Promise<CsConversation[]>;
  getHistory(
    conversationId: string,
    limit?: number,
    offset?: number,
  ): Promise<CsMessage[]>;
  sendMessage(conversationId: string, body: string): Promise<CsMessage>;
  sendMedia(
    conversationId: string,
    file: File,
    caption?: string,
  ): Promise<CsMessage>;
  assign(conversationId: string, userId: string): Promise<CsConversation>;
  setStatus(conversationId: string, status: "closed"): Promise<CsConversation>;
  linkOnt(conversationId: string, ontId: string | null): Promise<LinkOntResult>;
  getQuickReplies(): Promise<CsQuickReply[]>;
  createQuickReply(title: string, body: string): Promise<CsQuickReply>;
  updateQuickReply(
    id: string,
    title: string,
    body: string,
  ): Promise<CsQuickReply>;
  deleteQuickReply(id: string): Promise<void>;
  listWaAccounts(): Promise<WaAccount[]>;
  connectWaAccount(id: string, phone: string): Promise<{ status: string }>;
}
