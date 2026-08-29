export interface WireguardServer {
  id: string;
  interfaceName: string;
  listenPort: number;
  publicKey: string;
  endpointHost: string;
  tunnelSubnet: string;
  address: string;
}

export interface WireguardPeer {
  id: string;
  siteId: string;
  name: string;
  tunnelAddress: string;
  allowedIps: string[];
  persistentKeepalive: number;
  enabled: boolean;
  connected: boolean;
  lastHandshakeAt: string | null;
  rxBytes: number;
  txBytes: number;
  createdAt: string;
  updatedAt: string;
}

export interface WireguardPeerConfig {
  format: PeerConfigFormat;
  config: string;
}

export type PeerConfigFormat = "wg-quick" | "mikrotik";

export interface SaveWireguardServerDto {
  endpointHost: string;
  listenPort: number;
}

export interface CreateWireguardPeerDto {
  siteId: string;
  name: string;
  allowedIps: string[];
  tunnelAddress?: string;
}

export interface UpdateWireguardPeerDto {
  name?: string;
  allowedIps?: string[];
  enabled?: boolean;
}

export interface ReachabilityResult {
  reachable: boolean;
  /** False when the address is outside the subnets this tunnel carries. */
  routed: boolean;
  message: string;
}
