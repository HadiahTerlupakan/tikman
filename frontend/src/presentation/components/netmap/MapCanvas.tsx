import {
  AdvancedMarker,
  APIProvider,
  Map,
  Polyline,
} from "@vis.gl/react-google-maps";
import type {
  MappingEdge,
  MappingNode,
  NodeType,
  Waypoint,
} from "@/domain/entities";
import { NODE_COLORS } from "./mappingLabels";

// Where the map opens when there is nothing on it yet.
const FALLBACK_CENTER = { lat: -6.2, lng: 106.816 };
const FALLBACK_ZOOM = 13;
const NODE_ZOOM = 16;

// A saved cable reads as fixed infrastructure; the one still being traced
// needs to stand out from it while it is still only a proposal.
const EDGE_COLOR = "#f59e0b";
const DRAFT_COLOR = "#22c55e";

interface MapCanvasProps {
  nodes: MappingNode[];
  edges: MappingEdge[];
  /** The corners traced so far for the cable being drawn right now, if any. */
  draft: Waypoint[];
  /** What a map click means: place this kind of box, trace a cable, or nothing. */
  placing: NodeType | "cable" | undefined;
  apiKey: string;
  mapId?: string;
  onDrop: (point: Waypoint) => void;
  onNodeClick: (nodeId: string) => void;
}

/** Opens on what is already mapped, so a technician is never sent to the sea. */
function opening(nodes: MappingNode[]) {
  if (nodes.length === 0) {
    return { defaultCenter: FALLBACK_CENTER, defaultZoom: FALLBACK_ZOOM };
  }
  return {
    defaultCenter: { lat: nodes[0].latitude, lng: nodes[0].longitude },
    defaultZoom: NODE_ZOOM,
  };
}

/**
 * The full drawn length of one cable: its named ends plus the corners traced
 * between them. An edge stores only the corners — `useCableDraw` never records
 * the node it started or finished on — so even a straight drop with no corners
 * at all needs both ends resolved before it is a path at all.
 *
 * Migration 53 keeps no foreign key from edge to node on purpose: a cable can
 * be drawn before its ends are named, and deleting a node here leaves its
 * cables behind rather than cascading them away. `undefined` here means only
 * "skip this one edge" — never a thrown error that would blank the rest of
 * the map.
 */
export function edgePath(
  edge: MappingEdge,
  nodesById: Map<string, MappingNode>,
): Waypoint[] | undefined {
  const source = nodesById.get(edge.source);
  const target = nodesById.get(edge.target);
  if (!source || !target) {
    return undefined;
  }
  return [
    { lat: source.latitude, lng: source.longitude },
    ...(edge.waypoints ?? []),
    { lat: target.latitude, lng: target.longitude },
  ];
}

function NodeMarkers({
  nodes,
  onNodeClick,
}: {
  nodes: MappingNode[];
  onNodeClick: (nodeId: string) => void;
}) {
  return (
    <>
      {nodes.map((node) => (
        <AdvancedMarker
          key={node.nodeId}
          position={{ lat: node.latitude, lng: node.longitude }}
          title={node.name}
          onClick={() => onNodeClick(node.nodeId)}
        >
          <span
            style={{
              display: "block",
              width: 14,
              height: 14,
              borderRadius: "50%",
              background: NODE_COLORS[node.type],
              border: "2px solid #fff",
            }}
          />
        </AdvancedMarker>
      ))}
    </>
  );
}

/** Cables already saved, plus the path being traced right now, if any. */
function CableLines({
  nodes,
  edges,
  draft,
}: {
  nodes: MappingNode[];
  edges: MappingEdge[];
  draft: Waypoint[];
}) {
  // `Map` above is the imported map component, not the global constructor —
  // `globalThis` reaches past that shadowing to the real one.
  const nodesById = new globalThis.Map(
    nodes.map((node) => [node.nodeId, node]),
  );

  return (
    <>
      {edges.map((edge) => {
        const path = edgePath(edge, nodesById);
        if (!path) {
          return null;
        }
        return (
          <Polyline
            key={edge.edgeId}
            path={path}
            strokeColor={EDGE_COLOR}
            strokeWeight={3}
          />
        );
      })}
      {draft.length > 1 && (
        <Polyline path={draft} strokeColor={DRAFT_COLOR} strokeWeight={3} />
      )}
    </>
  );
}

/**
 * The map itself: boxes as coloured pins, cables as lines, and a click that
 * either places a box or extends the cable being traced, depending on what
 * the toolbar has armed. The click-to-corner tracing this hands off to is the
 * interaction described in the design spec — everything else here exists to
 * put it on a satellite map.
 */
export function MapCanvas({
  nodes,
  edges,
  draft,
  placing,
  apiKey,
  mapId,
  onDrop,
  onNodeClick,
}: MapCanvasProps) {
  return (
    <APIProvider apiKey={apiKey} libraries={["marker"]}>
      <Map
        mapId={mapId}
        {...opening(nodes)}
        mapTypeId="satellite"
        style={{ width: "100%", height: "60vh" }}
        gestureHandling="greedy"
        disableDefaultUI={false}
        onClick={(event) => {
          const latLng = event.detail.latLng;
          if (!latLng || !placing) {
            return;
          }
          onDrop({ lat: latLng.lat, lng: latLng.lng });
        }}
      >
        <NodeMarkers nodes={nodes} onNodeClick={onNodeClick} />
        <CableLines nodes={nodes} edges={edges} draft={draft} />
      </Map>
    </APIProvider>
  );
}
