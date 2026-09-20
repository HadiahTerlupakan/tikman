import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import type {
  ImportedEdge,
  ImportedNode,
  MappingEdge,
  MappingNode,
} from "@/domain/entities";
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

export function useUpdateEdge() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ edgeId, edge }: { edgeId: string; edge: MappingEdge }) =>
      repo.updateEdge(edgeId, edge),
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

// A mutation rather than a query: exporting does not read cached state, it
// fires a download each time it is asked for.
export function useExportMapping() {
  return useMutation({ mutationFn: () => repo.exportKmz() });
}

// Parses an uploaded file and reports what it found - nothing is written, so
// there is nothing here to invalidate.
export function usePreviewImport() {
  return useMutation({ mutationFn: (file: File) => repo.previewImport(file) });
}

export function useCommitImport() {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({
      nodes,
      edges,
    }: {
      nodes: ImportedNode[];
      edges: ImportedEdge[];
    }) => repo.commitImport(nodes, edges),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: NODES });
      qc.invalidateQueries({ queryKey: EDGES });
    },
  });
}
