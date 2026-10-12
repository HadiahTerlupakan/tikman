import type { WaAccount, WaAccountStatus } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";

/** The number's state as of right now: a live event beats the list this page
 * loaded with, which may be minutes old. */
export function liveStatus(
  account: WaAccount,
  stream: WaStreamStatus,
): WaAccountStatus {
  return stream[account.id]?.waStatus ?? account.status;
}
