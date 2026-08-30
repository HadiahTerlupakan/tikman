/** One known setting and whether it has a value. Never carries the value. */
export interface SettingStatus {
  name: string;
  label: string;
  description: string;
  configured: boolean;
  /** Masked, e.g. "AIza••••••••Y123". Empty when not configured. */
  preview: string;
  /** The stored value cannot be decrypted with the current ENCRYPTION_KEY. */
  unreadable?: boolean;
  updatedAt?: string;
}

/** Values whose features can only run in the browser, keyed by setting name. */
export type BrowserSettings = Record<string, string>;

export const GOOGLE_MAPS_API_KEY = "google_maps_api_key";
