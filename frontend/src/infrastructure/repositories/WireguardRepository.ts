import { apiClient } from "../http/apiClient";
import { API_ENDPOINTS } from "../http/endpoints";
import type {
  CreateWireguardPeerDto,
  PeerConfigFormat,
  SaveWireguardServerDto,
  UpdateWireguardPeerDto,
  WireguardPeer,
  WireguardPeerConfig,
  WireguardServer,
} from "@/domain/entities";

export class WireguardRepository {
  async getServer(): Promise<WireguardServer> {
    const response = await apiClient.get(API_ENDPOINTS.WIREGUARD_SERVER);
    return response.data;
  }

  async saveServer(data: SaveWireguardServerDto): Promise<WireguardServer> {
    const response = await apiClient.put(API_ENDPOINTS.WIREGUARD_SERVER, data);
    return response.data;
  }

  async getPeers(): Promise<WireguardPeer[]> {
    const response = await apiClient.get(API_ENDPOINTS.WIREGUARD_PEERS);
    return response.data;
  }

  async createPeer(data: CreateWireguardPeerDto): Promise<WireguardPeer> {
    const response = await apiClient.post(API_ENDPOINTS.WIREGUARD_PEERS, data);
    return response.data;
  }

  async updatePeer(
    id: string,
    data: UpdateWireguardPeerDto,
  ): Promise<WireguardPeer> {
    const response = await apiClient.put(
      API_ENDPOINTS.WIREGUARD_PEER_BY_ID(id),
      data,
    );
    return response.data;
  }

  async deletePeer(id: string): Promise<void> {
    await apiClient.delete(API_ENDPOINTS.WIREGUARD_PEER_BY_ID(id));
  }

  async getPeerConfig(
    id: string,
    format: PeerConfigFormat,
  ): Promise<WireguardPeerConfig> {
    const response = await apiClient.get(
      API_ENDPOINTS.WIREGUARD_PEER_CONFIG(id, format),
    );
    return response.data;
  }

  async getSuggestedSubnets(siteId: string): Promise<string[]> {
    const response = await apiClient.get(
      API_ENDPOINTS.WIREGUARD_SUGGESTED_SUBNETS(siteId),
    );
    return response.data.subnets;
  }
}
