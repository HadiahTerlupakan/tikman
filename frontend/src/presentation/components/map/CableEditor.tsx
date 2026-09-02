import { Button, Space, Tag } from "antd";
import { cableKindLabel, cableLengthLabel } from "./cableLabel";
import type { CableSegment } from "./cableSegments";

interface CableEditorProps {
  segment: CableSegment;
  drafting: boolean;
  draftCount: number;
  saving: boolean;
  onStartDraw: () => void;
  onSave: () => void;
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
  saving,
  onStartDraw,
  onSave,
  onCancel,
  onStraighten,
}: CableEditorProps) {
  if (drafting) {
    return (
      <Space wrap style={{ marginBottom: 12 }}>
        <Tag color="green">
          Klik di peta mengikuti tiang · {draftCount} titik
        </Tag>
        <Button type="primary" loading={saving} onClick={onSave}>
          Simpan jalur
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
