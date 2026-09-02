import { Button, Space, Tag } from "antd";
import { cableKindLabel, cableLengthLabel } from "./cableLabel";
import type { CableSegment } from "./cableSegments";

interface CableEditorProps {
  segment: CableSegment;
  drafting: boolean;
  draftCount: number;
  /** The path as traced so far, so its length can be read while drawing. */
  draftSegment?: CableSegment;
  saving: boolean;
  onStartDraw: () => void;
  onSave: () => void;
  onUndo: () => void;
  onCancel: () => void;
  onStraighten: () => void;
}

/**
 * What can be done with the cable that is selected.
 *
 * While tracing it says how many points are down, because the map itself gives
 * no other sign that a click landed.
 */
export function CableEditor({
  segment,
  drafting,
  draftCount,
  draftSegment,
  saving,
  onStartDraw,
  onSave,
  onUndo,
  onCancel,
  onStraighten,
}: CableEditorProps) {
  if (drafting) {
    return (
      <Space wrap style={{ marginBottom: 12 }}>
        <Tag color="green">
          Klik di peta mengikuti tiang · {draftCount} titik
        </Tag>
        {draftSegment && <Tag>{cableLengthLabel(draftSegment)}</Tag>}
        <Button type="primary" loading={saving} onClick={onSave}>
          Simpan jalur
        </Button>
        <Button disabled={draftCount === 0} onClick={onUndo}>
          Hapus titik terakhir
        </Button>
        <Button onClick={onCancel}>Batal</Button>
      </Space>
    );
  }

  return (
    <Space wrap style={{ marginBottom: 12 }}>
      <Tag>{cableKindLabel(segment)}</Tag>
      <Tag color={segment.traced ? "blue" : "default"}>
        {cableLengthLabel(segment)}
      </Tag>
      <Button onClick={onStartDraw}>Gambar jalur</Button>
      {segment.traced && (
        <Button loading={saving} onClick={onStraighten}>
          Kembali ke garis lurus
        </Button>
      )}
      <Button type="text" onClick={onCancel}>
        Tutup
      </Button>
    </Space>
  );
}
