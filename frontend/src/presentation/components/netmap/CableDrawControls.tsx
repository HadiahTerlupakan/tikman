import { Button, Space } from "antd";

interface CableDrawControlsProps {
  canUndo: boolean;
  onUndo: () => void;
  /** Only a redraw shows Selesai: a fresh cable finishes by tapping its
   * target node instead, and its endpoint is not fixed the way a redraw's is. */
  showFinish: boolean;
  onFinish: () => void;
}

/** The row shown while a cable is being traced, fresh or redrawn: undo the
 * last corner, and — for a redraw only — a way to finish explicitly. */
export function CableDrawControls({
  canUndo,
  onUndo,
  showFinish,
  onFinish,
}: CableDrawControlsProps) {
  return (
    <Space>
      <Button onClick={onUndo} disabled={!canUndo}>
        Batal titik
      </Button>
      {showFinish && (
        <Button type="primary" onClick={onFinish}>
          Selesai
        </Button>
      )}
    </Space>
  );
}
