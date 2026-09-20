import { Button, Segmented, Space } from "antd";
import type { NodeType } from "@/domain/entities";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";

export type MapView = "map" | "list";

// "olt" is a placing mode distinct from every NodeType: it never creates a
// mapping row by itself. It writes a position onto an existing OLT and lets
// the backend mirror it (OltPlacementModal, syncOLTMapNode) — the map never
// invents the record the way placing a NodeType does.
export type Placing = NodeType | "cable" | "olt";

interface MapToolbarProps {
  placing: Placing | undefined;
  onPlace: (type: NodeType) => void;
  onPlaceOlt: () => void;
  onDrawCable: () => void;
  onCancel: () => void;
  view: MapView;
  onView: (view: MapView) => void;
}

// Read off NODE_LABELS (a Record<NodeType, string>) rather than listed by
// hand: a fifth NodeType would fail that Record's own type check at compile
// time, so this can't go stale the way a separately hand-written array could.
// "server" is excluded on purpose — hand-placing one is the duplicate-OLT
// concept this feature removes, replaced by the dedicated OLT button below.
const PLACEABLE = (Object.keys(NODE_LABELS) as NodeType[]).filter(
  (type) => type !== "server",
);

/**
 * Controls above the map: what to place next, or how to get out of placing.
 *
 * Placing is modal — one tap on the map consumes it — so while it is active
 * the other kinds of box would just be noise; the only button that belongs is
 * the way out.
 */
export function MapToolbar({
  placing,
  onPlace,
  onPlaceOlt,
  onDrawCable,
  onCancel,
  view,
  onView,
}: MapToolbarProps) {
  return (
    <Space
      wrap
      style={{ width: "100%", justifyContent: "space-between", padding: 8 }}
    >
      <Space wrap>
        {placing ? (
          <Button danger onClick={onCancel}>
            Batal
          </Button>
        ) : (
          <>
            <Button
              type="primary"
              style={{ background: NODE_COLORS.server }}
              onClick={onPlaceOlt}
            >
              + OLT
            </Button>
            {PLACEABLE.map((type) => (
              <Button
                key={type}
                type="primary"
                style={{ background: NODE_COLORS[type] }}
                onClick={() => onPlace(type)}
              >
                + {NODE_LABELS[type]}
              </Button>
            ))}
            <Button onClick={onDrawCable}>Tarik kabel</Button>
          </>
        )}
      </Space>
      <Segmented
        value={view}
        onChange={(v) => onView(v as MapView)}
        options={[
          { label: "Peta", value: "map" },
          { label: "Daftar", value: "list" },
        ]}
      />
    </Space>
  );
}
