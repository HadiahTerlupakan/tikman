import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IWaGroupRepository } from "@/domain/repositories";
import type {
  WaGroup,
  WaGroupAction,
  WaGroupActionRequest,
  WaGroupDetail,
  WaGroupFilter,
  WaGroupMessage,
  WaGroupPollRequest,
} from "@/domain/entities";

/**
 * WaGroupRepository reaches the WhatsApp groups, which the API serves apart
 * from the customer inbox. Every handler wraps its payload as { data: ... },
 * as the CS handlers do.
 */
export class WaGroupRepository implements IWaGroupRepository {
  async list(filter?: WaGroupFilter): Promise<WaGroup[]> {
    const response = await apiClient.get(API_ENDPOINTS.CS_GROUPS, {
      params: {
        wa_account_id: filter?.waAccountId,
        search: filter?.search || undefined,
      },
    });
    return response.data.data ?? [];
  }

  async get(groupId: string): Promise<WaGroupDetail> {
    const response = await apiClient.get(API_ENDPOINTS.CS_GROUP(groupId));
    const detail = response.data.data;
    return { group: detail.group, participants: detail.participants ?? [] };
  }

  async getHistory(
    groupId: string,
    limit?: number,
    offset?: number,
  ): Promise<WaGroupMessage[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.CS_GROUP_MESSAGES(groupId),
      { params: { limit, offset } },
    );
    return response.data.data ?? [];
  }

  async sendMessage(
    groupId: string,
    body: string,
    replyToId?: string,
    mentions?: string[],
  ): Promise<WaGroupMessage> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_GROUP_MESSAGES(groupId),
      { body, replyToId, mentions },
    );
    return response.data.data;
  }

  async sendMedia(
    groupId: string,
    file: File,
    caption?: string,
    replyToId?: string,
    mentions: string[] = [],
  ): Promise<WaGroupMessage> {
    const form = new FormData();
    form.append("file", file);
    if (caption) {
      form.append("caption", caption);
    }
    for (const jid of mentions) {
      form.append("mentions", jid);
    }

    // In the query string for the reason CsRepository.sendMedia gives: the
    // body sits behind a size guard that a form field would have to precede.
    const url = replyToId
      ? `${API_ENDPOINTS.CS_GROUP_MEDIA_UPLOAD(groupId)}?reply_to_id=${replyToId}`
      : API_ENDPOINTS.CS_GROUP_MEDIA_UPLOAD(groupId);

    // Dropping the header hands the multipart boundary to the browser.
    const response = await apiClient.post(url, form, {
      headers: { "Content-Type": false },
    });
    return response.data.data;
  }

  /** Fire-and-forget: the list is re-read from WhatsApp in the background and
   * arrives as a group event, not in this response. */
  async refresh(waAccountId?: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.CS_GROUPS_REFRESH, { waAccountId });
  }

  async create(
    waAccountId: string,
    name: string,
    participants: string[],
  ): Promise<WaGroupAction> {
    const response = await apiClient.post(API_ENDPOINTS.CS_GROUPS, {
      waAccountId,
      name,
      participants,
    });
    return response.data.data;
  }

  async act(
    groupId: string,
    request: WaGroupActionRequest,
  ): Promise<WaGroupAction> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_GROUP_ACTIONS(groupId),
      request,
    );
    return response.data.data;
  }

  async getAction(actionId: string): Promise<WaGroupAction> {
    const response = await apiClient.get(
      API_ENDPOINTS.CS_GROUP_ACTION(actionId),
    );
    return response.data.data;
  }

  async setPinned(groupId: string, pinned: boolean): Promise<WaGroup> {
    const response = await apiClient.put(API_ENDPOINTS.CS_GROUP_PIN(groupId), {
      pinned,
    });
    return response.data.data;
  }

  async setTyping(groupId: string, typing: boolean): Promise<void> {
    await apiClient.post(API_ENDPOINTS.CS_GROUP_TYPING(groupId), { typing });
  }

  async requestHistory(groupId: string): Promise<WaGroupAction> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_GROUP_HISTORY_REQUEST(groupId),
    );
    return response.data.data;
  }

  async getStarred(groupId: string): Promise<WaGroupMessage[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.CS_GROUP_STARRED(groupId),
    );
    return response.data.data ?? [];
  }

  async createPoll(
    groupId: string,
    poll: WaGroupPollRequest,
  ): Promise<WaGroupMessage> {
    const response = await apiClient.post(
      API_ENDPOINTS.CS_GROUP_POLLS(groupId),
      poll,
    );
    return response.data.data;
  }
}
