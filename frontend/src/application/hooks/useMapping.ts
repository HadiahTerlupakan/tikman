import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { MappingRepository } from "@/infrastructure/repositories";

const repo = new MappingRepository();
const NODES = ["mapping", "nodes"];
const EDGES = ["mapping", "edges"];

export function useMappingNodes() {
  return useQuery({ queryKey: NODES, queryFn: () => repo.listNodes() });
}

export function useMappingEdges() {
  return useQuery({ queryKey: EDGES, queryFn: () => repo.listEdges() });
}

export function useCreateNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (node: MappingNode) => repo.createNode(node),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useUpdateNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ nodeId, node }: { nodeId: string; node: MappingNode }) =>
      repo.updateNode(nodeId, node),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useDeleteNode() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (nodeId: string) => repo.deleteNode(nodeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: NODES }),
  });
}

export function useCreateEdge() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (edge: MappingEdge) => repo.createEdge(edge),
    onSuccess: () => qc.invalidateQueries({ queryKey: EDGES }),
  });
}

export function useDeleteEdge() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: (edgeId: string) => repo.deleteEdge(edgeId),
    onSuccess: () => qc.invalidateQueries({ queryKey: EDGES }),
  });
}
