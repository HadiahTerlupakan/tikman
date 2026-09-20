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

/**
 * One Point placemark read back from an uploaded KMZ, resolved as far as the
 * backend safely can without a person's say-so - see backend
 * ImportedNode. `type` is `""` when neither the file's ExtendedData, its
 * folder nor its own name said anything usable; nothing is written to the
 * map until `include` is true and the row is sent to commitImport.
 */
export interface ImportedNode {
  row: number;
  nodeId: string;
  type: NodeType | "";
  name: string;
  latitude: number;
  longitude: number;
  capacity: number;
  splitter: string;
  pppoe: string;
  serialNumber: string;
  notes: string;
  /** Where `type` (and a guessed `nodeId`) came from - shown so a guess never reads as a fact. */
  reason: string;
  /** `nodeId` already names something on the map, or repeats within this file. */
  conflict: boolean;
  conflictReason: string;
  /** A "server" node mirrors a real OLT and can never be created by import - see the OLT menu instead. */
  blocked: boolean;
  blockedReason: string;
  include: boolean;
}

/** One LineString placemark read back from an uploaded KMZ - see backend ImportedEdge. */
export interface ImportedEdge {
  row: number;
  edgeId: string;
  source: string;
  target: string;
  fiberType: FiberType | "";
  distance: number;
  waypoints: Waypoint[] | null;
  notes: string;
  reason: string;
  conflict: boolean;
  conflictReason: string;
  /** `edgeId`, `source` or `target` could not be filled in at all. */
  unresolved: boolean;
  include: boolean;
}

/** A placemark that was neither a usable node nor a usable cable. */
export interface ImportIssue {
  row: number;
  name: string;
  folder: string;
  reason: string;
}

/** What an uploaded KMZ resolves to before anyone has confirmed anything. */
export interface ImportPreview {
  nodes: ImportedNode[];
  edges: ImportedEdge[];
  issues: ImportIssue[];
  totalPlacemarks: number;
}

/** What actually got written once the person confirmed the preview. */
export interface ImportResult {
  nodesCreated: number;
  edgesCreated: number;
}
