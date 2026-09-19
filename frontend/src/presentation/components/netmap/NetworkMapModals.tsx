import type {
  FiberType,
  MappingEdge,
  MappingNode,
  NodeType,
  Waypoint,
} from "@/domain/entities";
import { CableTypeModal } from "./CableTypeModal";
import { EdgeFormModal } from "./EdgeFormModal";
import { NodeFormModal } from "./NodeFormModal";

// What the node form is doing right now: adding a freshly tapped point.
// `initial` is set later (editing) once the list view or a map popup opens it.
export interface NodeFormTarget {
  type: NodeType;
  position: Waypoint;
  initial?: MappingNode;
}

// A cable with both ends named and its path traced, waiting only on which of
// the seven fiber types it is before it can be saved.
export interface PendingCable {
  source: string;
  target: string;
  waypoints: Waypoint[];
  distance: number;
}

interface NetworkMapModalsProps {
  formTarget?: NodeFormTarget;
  onCancelForm: () => void;
  onSubmitNode: (node: MappingNode) => void;
  pendingCable?: PendingCable;
  onCancelCable: () => void;
  onSubmitCable: (fiberType: FiberType) => void;
  editingEdge?: MappingEdge;
  onCancelEdge: () => void;
  onSubmitEdge: (edge: MappingEdge) => void;
}

/**
 * The one modal open on the map page at a time, if any: placing or editing a
 * node, picking a freshly traced cable's fiber type, or editing an existing
 * cable's details. Split out of NetworkMapPage to keep that file under the
 * project's line limit — these three are wiring, not page-level state.
 */
export function NetworkMapModals({
  formTarget,
  onCancelForm,
  onSubmitNode,
  pendingCable,
  onCancelCable,
  onSubmitCable,
  editingEdge,
  onCancelEdge,
  onSubmitEdge,
}: NetworkMapModalsProps) {
  return (
    <>
      {formTarget && (
        <NodeFormModal
          open
          type={formTarget.type}
          position={formTarget.position}
          initial={formTarget.initial}
          onCancel={onCancelForm}
          onSubmit={onSubmitNode}
        />
      )}
      {pendingCable && (
        <CableTypeModal
          open
          onCancel={onCancelCable}
          onSubmit={onSubmitCable}
        />
      )}
      {editingEdge && (
        <EdgeFormModal
          open
          initial={editingEdge}
          onCancel={onCancelEdge}
          onSubmit={onSubmitEdge}
        />
      )}
    </>
  );
}
