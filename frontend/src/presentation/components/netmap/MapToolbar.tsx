import { DownloadOutlined } from "@ant-design/icons";
import { Button, Segmented, Space } from "antd";
import type { NodeType } from "@/domain/entities";
import { ImportKmzButton } from "./ImportKmzButton";
import { NODE_COLORS, NODE_LABELS } from "./mappingLabels";
import { useKmzExport } from "./useKmzExport";

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

type PlacingButtonsProps = Pick<
  MapToolbarProps,
  "placing" | "onPlace" | "onPlaceOlt" | "onDrawCable" | "onCancel"
>;

// Placing is modal — one tap on the map consumes it — so while it is active
// the other kinds of box would just be noise; the only button that belongs is
// the way out.
function PlacingButtons({
  placing,
  onPlace,
  onPlaceOlt,
  onDrawCable,
  onCancel,
}: PlacingButtonsProps) {
  if (placing) {
    return (
      <Button danger onClick={onCancel}>
        Batal
      </Button>
    );
  }
  return (
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
  );
}

/**
 * Controls above the map: what to place next, how to get out of placing, and
 * (regardless of placing state) downloading the map as it stands or
 * switching between the map and list views.
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
  const { handleExport, isPending } = useKmzExport();

  return (
    <Space
      wrap
      style={{ width: "100%", justifyContent: "space-between", padding: 8 }}
    >
      <Space wrap>
        <PlacingButtons
          placing={placing}
          onPlace={onPlace}
          onPlaceOlt={onPlaceOlt}
          onDrawCable={onDrawCable}
          onCancel={onCancel}
        />
      </Space>
      <Space wrap>
        <Button
          icon={<DownloadOutlined />}
          loading={isPending}
          onClick={handleExport}
        >
          Unduh KMZ
        </Button>
        <ImportKmzButton />
        <Segmented
          value={view}
          onChange={(v) => onView(v as MapView)}
          options={[
            { label: "Peta", value: "map" },
            { label: "Daftar", value: "list" },
          ]}
        />
      </Space>
    </Space>
  );
}
