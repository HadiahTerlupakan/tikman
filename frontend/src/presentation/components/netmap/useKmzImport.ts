import { useState } from "react";
import { message } from "antd";
import type {
  ImportedEdge,
  ImportedNode,
  ImportPreview,
} from "@/domain/entities";
import { usePreviewImport, useCommitImport } from "@/application/hooks";

function importErrorMessage(error: unknown): string {
  // The backend's own text is already the plain-language explanation (a
  // stale collision, a full cabinet) - mappingErrors.ts's errorMessage
  // exists too, but its hardcoded fallback ("Gagal menyimpan kabel") is
  // worded for a single cable, which reads oddly for a batch that may be
  // only nodes.
  return error instanceof Error ? error.message : "Gagal menyimpan hasil impor";
}

// Shared by updateNode/updateEdge below: both only ever replace one row,
// found by the position classifyPlacemarks gave it on the backend, not by
// array index (which shifts if a row is ever added or removed).
function patchByRow<T extends { row: number }>(
  list: T[],
  row: number,
  patch: Partial<T>,
): T[] {
  return list.map((item) => (item.row === row ? { ...item, ...patch } : item));
}

// Shared by runPreview and commit below: both are "call the backend, then
// either apply what it returned or report why it refused" - the one
// difference (commit also resets the form on success) is left to the
// caller's own onSuccess rather than duplicated here.
async function runMutation<T>(
  fn: () => Promise<T>,
  onSuccess: (result: T) => void,
): Promise<boolean> {
  try {
    onSuccess(await fn());
    return true;
  } catch (error) {
    message.error(importErrorMessage(error));
    return false;
  }
}

/**
 * Orchestrates the whole import flow: pick a file, preview it, let the
 * person correct whatever the preview got wrong, then commit exactly that.
 * Nothing here writes to the map until commit() is called - see
 * MappingService.PreviewImport/CommitImport on the backend for why that
 * split exists at all.
 */
export function useKmzImport() {
  const previewMutation = usePreviewImport();
  const commitMutation = useCommitImport();
  const [preview, setPreview] = useState<ImportPreview>();

  const runPreview = (file: File) =>
    runMutation(() => previewMutation.mutateAsync(file), setPreview);

  const updateNode = (row: number, patch: Partial<ImportedNode>) =>
    setPreview((p) =>
      p ? { ...p, nodes: patchByRow(p.nodes, row, patch) } : p,
    );

  const updateEdge = (row: number, patch: Partial<ImportedEdge>) =>
    setPreview((p) =>
      p ? { ...p, edges: patchByRow(p.edges, row, patch) } : p,
    );

  const reset = () => setPreview(undefined);

  const commit = () => {
    if (!preview) {
      return Promise.resolve(false);
    }
    return runMutation(
      () =>
        commitMutation.mutateAsync({
          nodes: preview.nodes,
          edges: preview.edges,
        }),
      (result) => {
        message.success(
          `${result.nodesCreated} node dan ${result.edgesCreated} kabel disimpan ke peta`,
        );
        reset();
      },
    );
  };

  return {
    preview,
    runPreview,
    updateNode,
    updateEdge,
    commit,
    reset,
    isPreviewing: previewMutation.isPending,
    isCommitting: commitMutation.isPending,
  };
}
