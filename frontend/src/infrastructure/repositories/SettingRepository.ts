import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type { BrowserSettings, SettingStatus } from "@/domain/entities";

export class SettingRepository {
  async list(): Promise<SettingStatus[]> {
    const response = await apiClient.get(API_ENDPOINTS.SETTINGS);
    return response.data.data ?? [];
  }

  async save(name: string, value: string): Promise<SettingStatus[]> {
    const response = await apiClient.put(API_ENDPOINTS.SETTING_BY_NAME(name), {
      value,
    });
    return response.data.data ?? [];
  }

  async remove(name: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.SETTING_BY_NAME(name));
  }

  // Answers {} when nothing is configured, which is a normal state rather than
  // an error, so this must not throw on an empty installation.
  async browser(): Promise<BrowserSettings> {
    const response = await apiClient.get(API_ENDPOINTS.SETTINGS_BROWSER);
    return response.data ?? {};
  }
}
