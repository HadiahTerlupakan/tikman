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

  // The server returns {values: [{name, value}]} rather than a map keyed by
  // setting name: the axios interceptor runs every response through humps'
  // camelizeKeys, which would silently rewrite a name like
  // "google_maps_api_key" to "googleMapsApiKey" and break every lookup by
  // the snake_case constant. A list keeps "name" and "value" as the only
  // JSON keys — both already camelCase-safe — so the identifiers inside the
  // values ride through untouched.
  //
  // Answers an empty list when nothing is configured, which is a normal
  // state rather than an error, so this must not throw on an empty
  // installation.
  async browser(): Promise<BrowserSettings> {
    const response = await apiClient.get(API_ENDPOINTS.SETTINGS_BROWSER);
    const values: { name: string; value: string }[] =
      response.data?.values ?? [];
    return Object.fromEntries(values.map(({ name, value }) => [name, value]));
  }
}
