import type { FiberType, NodeType } from "@/domain/entities";

export const NODE_LABELS: Record<NodeType, string> = {
  server: "Server / OLT",
  odc: "ODC",
  odp: "ODP",
  ont: "ONT",
};

// One colour per kind, so a glance at the map says what is where.
export const NODE_COLORS: Record<NodeType, string> = {
  server: "#8b5cf6",
  odc: "#3b82f6",
  odp: "#06b6d4",
  ont: "#22c55e",
};

export const FIBER_LABELS: Record<FiberType, string> = {
  feeder: "Feeder",
  distribution: "Distribusi",
  drop: "Drop",
  odp_to_odp: "ODP ke ODP",
  odp_to_odp_ratio: "ODP ke ODP (splitter)",
  odc_to_odc: "ODC ke ODC",
  odc_to_odc_ratio: "ODC ke ODC (splitter)",
};
