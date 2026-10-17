import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IWaGroupMessageRepository } from "@/domain/repositories";
import type {
  WaGroupAction,
  WaGroupMessage,
  WaGroupReceipt,
} from "@/domain/entities";

/**
 * WaGroupMessageRepository acts on one message of a group. What WhatsApp has
 * to carry out (react, edit, revoke, vote) comes back as a queued action; what
 * only TikMan records (star, receipts) answers at once.
 */
export class WaGroupMessageRepository implements IWaGroupMessageRepository {
  private path(groupId: string, messageId: string, tail = ""): string {
    return `${API_ENDPOINTS.CS_GROUP_MESSAGE(groupId, messageId)}${tail}`;
  }

  async react(
    groupId: string,
    messageId: string,
    emoji: string,
  ): Promise<WaGroupAction> {
    const response = await apiClient.post(
      this.path(groupId, messageId, "/reaction"),
      { emoji },
    );
    return response.data.data;
  }

  async edit(
    groupId: string,
    messageId: string,
    body: string,
  ): Promise<WaGroupAction> {
    const response = await apiClient.put(this.path(groupId, messageId), {
      body,
    });
    return response.data.data;
  }

  async revoke(groupId: string, messageId: string): Promise<WaGroupAction> {
    const response = await apiClient.delete(this.path(groupId, messageId));
    return response.data.data;
  }

  async setStarred(
    groupId: string,
    messageId: string,
    starred: boolean,
  ): Promise<WaGroupMessage> {
    const response = await apiClient.put(
      this.path(groupId, messageId, "/star"),
      { starred },
    );
    return response.data.data;
  }

  async forward(
    groupId: string,
    messageId: string,
    groupIds: string[],
  ): Promise<WaGroupMessage[]> {
    const response = await apiClient.post(
      this.path(groupId, messageId, "/forward"),
      { groupIds },
    );
    return response.data.data ?? [];
  }

  async replyPrivately(groupId: string, messageId: string): Promise<string> {
    const response = await apiClient.post(
      this.path(groupId, messageId, "/reply-private"),
    );
    return response.data.data.conversationId;
  }

  async getReceipts(
    groupId: string,
    messageId: string,
  ): Promise<WaGroupReceipt[]> {
    const response = await apiClient.get(
      this.path(groupId, messageId, "/receipts"),
    );
    return response.data.data ?? [];
  }

  async vote(
    groupId: string,
    messageId: string,
    options: string[],
  ): Promise<WaGroupAction> {
    const response = await apiClient.post(
      this.path(groupId, messageId, "/vote"),
      { options },
    );
    return response.data.data;
  }
}
