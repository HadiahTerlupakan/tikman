import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { WireguardRepository } from "@/infrastructure/repositories";
import type {
  CreateWireguardPeerDto,
  PeerConfigFormat,
  SaveWireguardServerDto,
  UpdateWireguardPeerDto,
} from "@/domain/entities";

const repository = new WireguardRepository();

export function useWireguardServer() {
  return useQuery({
    queryKey: ["wireguard", "server"],
    queryFn: () => repository.getServer(),
    retry: false, // a missing server is the expected first-run state, not a failure
  });
}

export function useSaveWireguardServer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: SaveWireguardServerDto) => repository.saveServer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "server"] });
    },
  });
}

export function useWireguardPeers() {
  return useQuery({
    queryKey: ["wireguard", "peers"],
    queryFn: () => repository.getPeers(),
    refetchInterval: 30_000, // matches the API's status refresh cycle
  });
}

export function useCreateWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (data: CreateWireguardPeerDto) => repository.createPeer(data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useUpdateWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: ({ id, data }: { id: string; data: UpdateWireguardPeerDto }) =>
      repository.updatePeer(id, data),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useDeleteWireguardPeer() {
  const queryClient = useQueryClient();

  return useMutation({
    mutationFn: (id: string) => repository.deletePeer(id),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ["wireguard", "peers"] });
    },
  });
}

export function useSuggestedSubnets(siteId: string | undefined) {
  return useQuery({
    queryKey: ["wireguard", "suggested-subnets", siteId],
    queryFn: () => repository.getSuggestedSubnets(siteId as string),
    enabled: !!siteId,
  });
}

export function usePeerConfig() {
  return useMutation({
    mutationFn: ({ id, format }: { id: string; format: PeerConfigFormat }) =>
      repository.getPeerConfig(id, format),
  });
}
