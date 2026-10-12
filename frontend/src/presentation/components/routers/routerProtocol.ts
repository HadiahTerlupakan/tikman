import type { RouterApiProtocol } from "@/domain/entities";

// Must match mikrotik.Protocol.DefaultPort on the backend.
export const PROTOCOL_DEFAULT_PORT: Record<RouterApiProtocol, number> = {
  rest: 8443,
  "api-ssl": 8729,
  api: 8728,
};

export const PROTOCOL_LABEL: Record<RouterApiProtocol, string> = {
  rest: "REST",
  "api-ssl": "API-SSL",
  api: "API",
};

/** Plain API has no certificate, so there is nothing to confirm or pin. */
export function usesTls(protocol: RouterApiProtocol): boolean {
  return protocol !== "api";
}
