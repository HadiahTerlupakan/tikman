export type NodeType = "server" | "odc" | "odp" | "ont";

export type FiberType =
  | "feeder"
  | "distribution"
  | "drop"
  | "odp_to_odp"
  | "odp_to_odp_ratio"
  | "odc_to_odc"
  | "odc_to_odc_ratio";

export interface Waypoint {
  lat: number;
  lng: number;
}

export interface MappingNode {
  id?: string;
  nodeId: string;
  type: NodeType;
  name: string;
  latitude: number;
  longitude: number;
  capacity: number;
  splitter: string;
  pppoe: string;
  serialNumber: string;
  notes: string;
  /** Set only on a node that mirrors an OLT's own coordinates (backend:
   * MappingNode.OLTID). Absent on every node placed by hand. Name and type
   * on such a node are the OLT's, not the map's — the backend silently
   * discards edits to either. */
  oltId?: string;
}

export interface MappingEdge {
  id?: string;
  edgeId: string;
  source: string;
  target: string;
  fiberType: FiberType;
  distance: number;
  waypoints: Waypoint[] | null;
  notes: string;
}
