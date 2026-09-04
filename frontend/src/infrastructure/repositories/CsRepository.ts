import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { ICsRepository, LinkOntResult } from "@/domain/repositories";
import type {
  ChannelPost,
  CsConversation,
  CsConversationFilter,
  CsMessage,
  CsQuickReply,
  WaAccount,
  WaChannel,
} from "@/domain/entities";

/**
 * CsRepository reaches the shared WhatsApp inbox. Every handler on the
 * backend wraps its payload as { data: ... } (see cs_handler_*.go), so every
 * method here unwraps one level deeper than SiteRepository does.
 */
export class CsRepository implements ICsRepository {
  async getConversations(
    filter?: CsConversationFilter,
  ): Promise<CsConversation[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_CONVERSATIONS, {
      params: {
        search: filter?.search,
        limit: filter?.limit,
        offset: filter?.offset,
        mine: filter?.mine ? "true" : undefined,
        awaiting_reply: filter?.awaitingReply ? "true" : undefined,
        closed: filter?.closed ? "true" : undefined,
      },
    });
    return response.data.data ?? [];
  }

  async getHistory(
    conversationId: string,
    limit?: number,
    offset?: number,
  ): Promise<CsMessage[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.CS_MESSAGES(conversationId),
      {
        params: { limit, offset },
      },
    );
    return response.data.data ?? [];
  }

  async sendMessage(
    conversationId: string,
    body: string,
    replyToId?: string,
  ): Promise<CsMessage> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_MESSAGES(conversationId),
      { body, replyToId },
    );
    return response.data.data;
  }

  /** Raises or clears the "typing…" line on the customer's phone. Nothing is
   * stored and nothing comes back: a typing state is true for a few seconds
   * and then is not. */
  async setTyping(conversationId: string, typing: boolean): Promise<void> {
    await apiClient.post(API_ENDPOINTS.CS_TYPING(conversationId), { typing });
  }

  async sendMedia(
    conversationId: string,
    file: File,
    caption?: string,
    replyToId?: string,
  ): Promise<CsMessage> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }

    // The quoted message travels in the query string, not the form: the API
    // wraps the request body in a size guard before anything reads it, and a
    // form field would have to be parsed ahead of that guard.
    const url = replyToId
      ? `${API_ENDPOINTS.CS_MEDIA_UPLOAD(conversationId)}?reply_to_id=${replyToId}`
      : API_ENDPOINTS.CS_MEDIA_UPLOAD(conversationId);

    const response = await apiClient.post(
      url,
      form,
      // Dropping the header hands the boundary to the browser, which is the
      // only thing that knows it. Leaving the client's default
      // application/json in place would make axios JSON-encode the FormData
      // instead of sending it, and the file would never leave.
      { headers: { "Content-Type": false } },
    );
    return response.data.data;
  }

  async assign(
    conversationId: string,
    userId: string,
  ): Promise<CsConversation> {
    const response = await apiClient.put(
      API_ENDPOINTS.CS_ASSIGN(conversationId),
      { userId },
    );
    return response.data.data;
  }

  async setStatus(
    conversationId: string,
    status: "closed",
  ): Promise<CsConversation> {
    const response = await apiClient.put(
      API_ENDPOINTS.CS_STATUS(conversationId),
      { status },
    );
    return response.data.data;
  }

  async linkOnt(
    conversationId: string,
    ontId: string | null,
  ): Promise<LinkOntResult> {
    const response = await apiClient.put(
      API_ENDPOINTS.CS_LINK_ONT(conversationId),
      { ontId },
    );
    return {
      conversation: response.data.data,
      phoneRecorded: response.data.phoneRecorded,
    };
  }

  async getQuickReplies(): Promise<CsQuickReply[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_QUICK_REPLIES);
    return response.data.data ?? [];
  }

  async createQuickReply(title: string, body: string): Promise<CsQuickReply> {
    const response = await apiClient.post(API_ENDPOINTS.CS_QUICK_REPLIES, {
      title,
      body,
    });
    return response.data.data;
  }

  async updateQuickReply(
    id: string,
    title: string,
    body: string,
  ): Promise<CsQuickReply> {
    const response = await apiClient.put(
      API_ENDPOINTS.CS_QUICK_REPLY_BY_ID(id),
      { title, body },
    );
    return response.data.data;
  }

  async deleteQuickReply(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.CS_QUICK_REPLY_BY_ID(id));
  }

  async listWaAccounts(): Promise<WaAccount[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_WA_ACCOUNTS);
    return response.data.data ?? [];
  }

  async createWaAccount(label: string): Promise<WaAccount> {
    const response = await apiClient.post(API_ENDPOINTS.CS_WA_ACCOUNTS, {
      label,
    });
    return response.data.data;
  }

  async connectWaAccount(
    id: string,
    phone: string,
  ): Promise<{ status: string }> {
    const response = await apiClient.post(API_ENDPOINTS.CS_WA_CONNECT(id), {
      phone,
    });
    return response.data.data;
  }

  async disconnectWaAccount(id: string): Promise<{ status: string }> {
    const response = await apiClient.post(API_ENDPOINTS.CS_WA_DISCONNECT(id));
    return response.data.data;
  }

  /** Removes a number along with every thread, message and file on it. */
  async deleteWaAccount(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.CS_WA_ACCOUNT_BY_ID(id));
  }

  async listWaChannels(): Promise<WaChannel[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_WA_CHANNELS);
    return response.data.data;
  }

  /** Fire-and-forget: the API asks the wa process to re-read its channel
   * lists and answers immediately. The result arrives as changed rows on the
   * next fetch, not in this response. */
  async refreshWaChannels(): Promise<void> {
    await apiClient.post(API_ENDPOINTS.CS_WA_CHANNELS_REFRESH);
  }

  async getChannelPosts(channelId: string): Promise<ChannelPost[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_CHANNEL_POSTS, {
      params: { channel_id: channelId },
    });
    return response.data.data;
  }

  async sendChannelPost(channelId: string, body: string): Promise<ChannelPost> {
    const response = await apiClient.post(API_ENDPOINTS.CS_CHANNEL_POSTS, {
      channel_id: channelId,
      body,
    });
    return response.data.data;
  }

  async sendChannelPostMedia(
    channelId: string,
    file: File,
    caption?: string,
  ): Promise<ChannelPost> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }

    // The channel travels in the query string, not the form, for the reason
    // sendMedia records: the API wraps the body in a size guard before
    // anything reads it, and a form field would have to be parsed ahead of it.
    const response = await apiClient.post(
      `${API_ENDPOINTS.CS_CHANNEL_POSTS_MEDIA}?channel_id=${channelId}`,
      form,
      // Dropping the header hands the boundary to the browser, which is the
      // only thing that knows it.
      { headers: { "Content-Type": false } },
    );
    return response.data.data;
  }

  async deleteMessage(id: string): Promise<number> {
    const response = await apiClient.delete(API_ENDPOINTS.CS_MESSAGE_BY_ID(id));
    return response.data.data?.removed ?? 0;
  }

  async clearConversation(conversationId: string): Promise<number> {
    const response = await apiClient.delete(
      API_ENDPOINTS.CS_MESSAGES(conversationId),
    );
    return response.data.data?.removed ?? 0;
  }

  async clearWaAccountMessages(id: string): Promise<number> {
    const response = await apiClient.delete(
      API_ENDPOINTS.CS_WA_ACCOUNT_MESSAGES(id),
    );
    return response.data.data?.removed ?? 0;
  }

  async clearInbox(): Promise<number> {
    const response = await apiClient.delete(API_ENDPOINTS.CS_ALL_MESSAGES);
    return response.data.data?.removed ?? 0;
  }
}
