export type WaAccountStatus =
  | "disconnected"
  | "pairing"
  | "connected"
  | "banned";

/**
 * One WhatsApp number the team answers from. The wa process creates a single
 * row on startup — there is no "create account" endpoint — so in practice
 * this list has exactly one entry.
 */
export interface WaAccount {
  id: string;
  label: string;
  jid?: string;
  status: WaAccountStatus;
  lastConnectedAt?: string;
  createdAt: string;
  updatedAt: string;
}
