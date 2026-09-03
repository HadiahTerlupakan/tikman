export type WaAccountStatus =
  | "disconnected"
  | "pairing"
  | "connected"
  | "banned";

/**
 * One WhatsApp number the team answers from. A fresh install is seeded with
 * one; an admin adds the rest, and every one of them is answered from the same
 * inbox — a reply leaves from the number that customer wrote to.
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
