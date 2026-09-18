import { ApiError } from "@/infrastructure/http";

// A capacity rule on an odp_to_odp/odc_to_odc cascade is also a 409, and must
// reach the operator as itself; only the id collision this cable's own
// `source--target` naming produces should read as "already exists". Branching
// on the code the backend now sends (not the shared status) is what tells
// them apart.
export function isEdgeExists(error: unknown): boolean {
  return error instanceof ApiError && error.code === "EDGE_EXISTS";
}

export function isNodeExists(error: unknown): boolean {
  return error instanceof ApiError && error.code === "NODE_EXISTS";
}

export function isNodeInUse(error: unknown): boolean {
  return error instanceof ApiError && error.code === "NODE_IN_USE";
}

export function errorMessage(error: unknown): string {
  return error instanceof Error ? error.message : "Gagal menyimpan kabel";
}
