import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { IPushRepository } from "@/domain/repositories";

export class PushRepository implements IPushRepository {
  async subscribe(fcmToken: string): Promise<void> {
    await apiClient.post(API_ENDPOINTS.PUSH_SUBSCRIBE, { fcmToken });
  }

  async unsubscribe(fcmToken: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.PUSH_SUBSCRIBE, {
      data: { fcmToken },
    });
  }
}
