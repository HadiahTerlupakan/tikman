import { InfoWindow } from "@vis.gl/react-google-maps";
import type { MappingEdge, MappingNode } from "@/domain/entities";
import { EdgePopup } from "./EdgePopup";
import { NodePopup } from "./NodePopup";

/** What each popup's actions do. */
export interface PopupActions {
  onEditNode: (node: MappingNode) => void;
  onDeleteNode: (nodeId: string) => void;
  onEditEdge: (edge: MappingEdge) => void;
  onRedrawEdge: (edge: MappingEdge) => void;
  onDeleteEdge: (edgeId: string) => void;
}

/** The node or cable whose popup is open, if any (mutually exclusive, set by
 * the page in response to onNodeClick/onEdgeClick), how to close it, and what
 * its actions do. Grouped into one object because all four fields exist for
 * exactly one reason — rendering the currently-open popup — and nowhere else
 * in MapCanvas; see the prop-shape note on MapCanvasProps.tracing for the
 * cluster that stayed flat instead. */
export interface PopupState {
  selectedNode?: MappingNode;
  selectedEdge?: MappingEdge;
  onClose: () => void;
  actions: PopupActions;
}

interface NodePopupWindowProps {
  node: MappingNode;
  edges: MappingEdge[];
  nodesById: Map<string, MappingNode>;
  onClose: () => void;
  actions: PopupActions;
}

function NodePopupWindow({
  node,
  edges,
  nodesById,
  onClose,
  actions,
}: NodePopupWindowProps) {
  return (
    <InfoWindow
      position={{ lat: node.latitude, lng: node.longitude }}
      onCloseClick={onClose}
    >
      <NodePopup
        node={node}
        edges={edges}
        nodesById={nodesById}
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
// the popup entirely only if neither does — there is nowhere left to anchor
// it. This is reachable in production, not just permitted by the schema: an
// already-open cable popup flips to a missing endpoint the moment
// useMappingNodes() refetches after that node is deleted in another tab —
// nodesById rebuilds from the live `nodes` prop on every render.
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
  /** Every cable on the map — passed straight through to a node's popup for
   * its slot usage and connection counts (see NodePopup). */
  edges: MappingEdge[];
  /** False while a cable is being traced: "no popup, no interference" covers
   * a selection left over from before tracing started, not just a fresh
   * click during it. */
  visible: boolean;
  popup: PopupState;
}

/** The one popup open on the map, if any. A node takes priority by
 * construction (useMapSelection never holds both at once), so checking it
 * first is enough rather than a rule that needs stating separately. */
export function SelectedPopups({
  nodesById,
  edges,
  visible,
  popup,
}: SelectedPopupsProps) {
  if (!visible) {
    return null;
  }
  if (popup.selectedNode) {
    return (
      <NodePopupWindow
        node={popup.selectedNode}
        edges={edges}
        nodesById={nodesById}
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
