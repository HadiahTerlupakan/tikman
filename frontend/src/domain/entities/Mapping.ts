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
