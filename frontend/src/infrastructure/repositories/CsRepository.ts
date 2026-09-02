import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { ICsRepository, LinkOntResult } from "@/domain/repositories";
import type {
  CsConversation,
  CsConversationFilter,
  CsMessage,
  CsQuickReply,
  WaAccount,
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
        unassigned: filter?.unassigned ? "true" : undefined,
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

  async sendMessage(conversationId: string, body: string): Promise<CsMessage> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_MESSAGES(conversationId),
      { body },
    );
    return response.data.data;
  }

  async sendMedia(
    conversationId: string,
    file: File,
    caption?: string,
  ): Promise<CsMessage> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }

    const response = await apiClient.post(
      API_ENDPOINTS.CS_MEDIA_UPLOAD(conversationId),
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

  async listWaAccounts(): Promise<WaAccount[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_WA_ACCOUNTS);
    return response.data.data ?? [];
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
}
