import {
  AdvancedMarker,
  APIProvider,
  InfoWindow,
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
import { EdgePopup } from "./EdgePopup";
import { NODE_COLORS } from "./mappingLabels";
import { NodePopup } from "./NodePopup";

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

/** What each popup's actions do. */
interface PopupActions {
  onEditNode: (node: MappingNode) => void;
  onDeleteNode: (nodeId: string) => void;
  onEditEdge: (edge: MappingEdge) => void;
  onRedrawEdge: (edge: MappingEdge) => void;
  onDeleteEdge: (edgeId: string) => void;
}

/** The node or cable whose popup is open, if any (mutually exclusive, set by
 * the page in response to onNodeClick/onEdgeClick), how to close it, and what
 * its actions do. Grouped into one object rather than eight flat props —
 * MapCanvas already carries the map's own props, and threading these
 * individually would make its signature a parameter list rather than a
 * component's inputs. */
interface PopupState {
  selectedNode?: MappingNode;
  selectedEdge?: MappingEdge;
  onClose: () => void;
  actions: PopupActions;
}

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
  onEdgeClick: (edgeId: string) => void;
  popup: PopupState;
}

/** A click on the map background places a box or extends a cable, depending
 * on what is armed; with nothing armed, a bare click is not a drop. */
function dropAt(
  latLng: { lat: number; lng: number } | null,
  placing: NodeType | "cable" | undefined,
  onDrop: (point: Waypoint) => void,
) {
  if (!latLng || !placing) {
    return;
  }
  onDrop({ lat: latLng.lat, lng: latLng.lng });
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

interface CableLinesProps {
  nodesById: Map<string, MappingNode>;
  edges: MappingEdge[];
  draft: Waypoint[];
  fromNodeId?: string;
  redrawingEdgeId?: string;
  placing: NodeType | "cable" | undefined;
  onEdgeClick: (edgeId: string) => void;
}

/** Cables already saved, plus the path being traced right now, if any. */
function CableLines({
  nodesById,
  edges,
  draft,
  fromNodeId,
  redrawingEdgeId,
  placing,
  onEdgeClick,
}: CableLinesProps) {
  // The saved path always starts at the source node (edgePath does the same
  // walk); the line still being traced has to match that, or a technician
  // sights it against the wrong start point while pulling fibre.
  const fromNode = fromNodeId ? nodesById.get(fromNodeId) : undefined;
  const draftPath = fromNode
    ? [{ lat: fromNode.latitude, lng: fromNode.longitude }, ...draft]
    : draft;

  // Attached only outside tracing: a handler that merely no-op'd while
  // tracing would still capture the click, stopping it from ever reaching
  // the map underneath (how a technician drops a corner on an existing line).
  const clickable = placing !== "cable";

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
            onClick={clickable ? () => onEdgeClick(edge.edgeId) : undefined}
          />
        );
      })}
      {draftPath.length > 1 && (
        <Polyline path={draftPath} strokeColor={DRAFT_COLOR} strokeWeight={3} />
      )}
    </>
  );
}

interface NodePopupWindowProps {
  node: MappingNode;
  onClose: () => void;
  actions: PopupActions;
}

function NodePopupWindow({ node, onClose, actions }: NodePopupWindowProps) {
  return (
    <InfoWindow
      position={{ lat: node.latitude, lng: node.longitude }}
      onCloseClick={onClose}
    >
      <NodePopup
        node={node}
        onEdit={actions.onEditNode}
        onDelete={actions.onDeleteNode}
        onClose={onClose}
      />
    </InfoWindow>
  );
}

interface EdgePopupWindowProps {
  edge: MappingEdge;
  nodesById: Map<string, MappingNode>;
  onClose: () => void;
  actions: PopupActions;
}

// Node deletion never cascades to edges (migration 53's own design), so
// either end can be gone; anchor on whichever one still resolves, and skip
// the popup entirely only if neither does — there is nowhere left to anchor it.
function EdgePopupWindow({
  edge,
  nodesById,
  onClose,
  actions,
}: EdgePopupWindowProps) {
  const source = nodesById.get(edge.source);
  const target = nodesById.get(edge.target);
  const anchor = source ?? target;
  if (!anchor) {
    return null;
  }
  return (
    <InfoWindow
      position={{ lat: anchor.latitude, lng: anchor.longitude }}
      onCloseClick={onClose}
    >
      <EdgePopup
        edge={edge}
        sourceNode={source}
        targetNode={target}
        onEdit={actions.onEditEdge}
        onRedraw={actions.onRedrawEdge}
        onDelete={actions.onDeleteEdge}
        onClose={onClose}
      />
    </InfoWindow>
  );
}

interface SelectedPopupsProps {
  nodesById: Map<string, MappingNode>;
  /** False while a cable is being traced: "no popup, no interference" covers
   * a selection left over from before tracing started, not just a fresh
   * click during it. */
  visible: boolean;
  popup: PopupState;
}

/** The one popup open on the map, if any. A node takes priority by
 * construction (useMapSelection never holds both at once), so checking it
 * first is enough rather than a rule that needs stating separately. */
function SelectedPopups({ nodesById, visible, popup }: SelectedPopupsProps) {
  if (!visible) {
    return null;
  }
  if (popup.selectedNode) {
    return (
      <NodePopupWindow
        node={popup.selectedNode}
        onClose={popup.onClose}
        actions={popup.actions}
      />
    );
  }
  if (!popup.selectedEdge) {
    return null;
  }
  return (
    <EdgePopupWindow
      edge={popup.selectedEdge}
      nodesById={nodesById}
      onClose={popup.onClose}
      actions={popup.actions}
    />
  );
}

/**
 * The map itself: boxes as coloured pins, cables as lines, and a click that
 * either places a box, extends the cable being traced, or opens a popup,
 * depending on what the toolbar has armed. The click-to-corner tracing this
 * hands off to is the interaction described in the design spec — everything
 * else here exists to put it on a satellite map.
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
  onEdgeClick,
  popup,
}: MapCanvasProps) {
  // `Map` below is the imported map component, not the global constructor —
  // `globalThis` reaches past that shadowing to the real one.
  const nodesById = new globalThis.Map(nodes.map((n) => [n.nodeId, n]));

  return (
    <APIProvider apiKey={apiKey} libraries={["marker"]}>
      <Map
        mapId={mapId}
        {...opening(nodes)}
        mapTypeId="satellite"
        style={{ width: "100%", height: "60vh" }}
        gestureHandling="greedy"
        disableDefaultUI={false}
        onClick={(event) => dropAt(event.detail.latLng, placing, onDrop)}
      >
        <NodeMarkers nodes={nodes} onNodeClick={onNodeClick} />
        <CableLines
          nodesById={nodesById}
          edges={edges}
          draft={draft}
          fromNodeId={fromNodeId}
          redrawingEdgeId={redrawingEdgeId}
          placing={placing}
          onEdgeClick={onEdgeClick}
        />
        <SelectedPopups
          nodesById={nodesById}
          visible={placing !== "cable"}
          popup={popup}
        />
      </Map>
    </APIProvider>
  );
}
