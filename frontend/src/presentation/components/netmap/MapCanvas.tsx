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
import { edgePath } from "./cableMath";
import { NODE_COLORS } from "./mappingLabels";

// Where the map opens when there is nothing on it yet.
const FALLBACK_CENTER = { lat: -6.2, lng: 106.816 };
const FALLBACK_ZOOM = 13;
const NODE_ZOOM = 16;

// A saved cable reads as fixed infrastructure; the one still being traced
// needs to stand out from it while it is still only a proposal. The cable
// being redrawn is still saved infrastructure too, but it is about to be
// replaced — a third colour is what lets a technician tell "the route I'm
// correcting" apart from every other cable on the map, not just from the
// new line.
const EDGE_COLOR = "#f59e0b";
const DRAFT_COLOR = "#22c55e";
const REDRAWING_COLOR = "#94a3b8";

interface MapCanvasProps {
  nodes: MappingNode[];
  edges: MappingEdge[];
  /** The corners traced so far for the cable being drawn right now, if any. */
  draft: Waypoint[];
  /** The node the cable being traced started from, so the in-progress line
   * can begin there instead of at its first corner — the same start point
   * edgePath gives a saved cable. */
  fromNodeId?: string;
  /** The cable whose route is being redrawn, if any — kept on the map in a
   * distinct colour so a technician can see what they are correcting while
   * the new line is traced over it. */
  redrawingEdgeId?: string;
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
  fromNodeId,
  redrawingEdgeId,
}: {
  nodes: MappingNode[];
  edges: MappingEdge[];
  draft: Waypoint[];
  fromNodeId?: string;
  redrawingEdgeId?: string;
}) {
  // `Map` above is the imported map component, not the global constructor —
  // `globalThis` reaches past that shadowing to the real one.
  const nodesById = new globalThis.Map(
    nodes.map((node) => [node.nodeId, node]),
  );

  // The saved path always starts at the source node (edgePath does the same
  // walk); the line still being traced has to match that, or a technician
  // sights it against the wrong start point while pulling fibre.
  const fromNode = fromNodeId ? nodesById.get(fromNodeId) : undefined;
  const draftPath = fromNode
    ? [{ lat: fromNode.latitude, lng: fromNode.longitude }, ...draft]
    : draft;

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
            strokeColor={
              edge.edgeId === redrawingEdgeId ? REDRAWING_COLOR : EDGE_COLOR
            }
            strokeWeight={3}
          />
        );
      })}
      {draftPath.length > 1 && (
        <Polyline path={draftPath} strokeColor={DRAFT_COLOR} strokeWeight={3} />
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
  fromNodeId,
  redrawingEdgeId,
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
        <CableLines
          nodes={nodes}
          edges={edges}
          draft={draft}
          fromNodeId={fromNodeId}
          redrawingEdgeId={redrawingEdgeId}
        />
      </Map>
    </APIProvider>
  );
}
