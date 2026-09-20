import type {
  FiberType,
  MappingEdge,
  MappingNode,
  NodeType,
  Olt,
  Waypoint,
} from "@/domain/entities";
import { CableTypeModal } from "./CableTypeModal";
import { EdgeFormModal } from "./EdgeFormModal";
import { NodeFormModal } from "./NodeFormModal";
import { OltPlacementModal } from "./OltPlacementModal";

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

// What the OLT-placement form is doing right now: a map tap fixed a
// position, and only which OLT it belongs to remains to be asked.
export interface OltPlacementTarget {
  position: Waypoint;
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
  oltPlacement?: OltPlacementTarget;
  /** OLTs with no coordinates yet — the only ones there is anything to place. */
  unplacedOlts: Olt[];
  onCancelOltPlacement: () => void;
  onSubmitOltPlacement: (oltId: string) => void;
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
  oltPlacement,
  unplacedOlts,
  onCancelOltPlacement,
  onSubmitOltPlacement,
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
      {oltPlacement && (
        <OltPlacementModal
          open
          position={oltPlacement.position}
          olts={unplacedOlts}
          onCancel={onCancelOltPlacement}
          onSubmit={onSubmitOltPlacement}
        />
      )}
    </>
  );
}
