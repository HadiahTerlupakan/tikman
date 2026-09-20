import { Descriptions, Form, Modal, Select } from "antd";
import type { Olt, Waypoint } from "@/domain/entities";

interface OltPlacementModalProps {
  open: boolean;
  /** Where the map was tapped — already fixed before this form opens. */
  position: Waypoint;
  /** OLTs with no coordinates yet; the only ones there is anything to place. */
  olts: Olt[];
  onCancel: () => void;
  onSubmit: (oltId: string) => void;
}

interface OltPlacementValues {
  oltId: string;
}

/**
 * Completes an OLT placement the way NodeFormModal completes any other: the
 * tap already fixed the position, so the only thing left to ask is which
 * OLT — never a name or a type, both of which the OLT record already owns
 * and the backend would silently discard here anyway (see MappingNode.oltId).
 */
export function OltPlacementModal({
  open,
  position,
  olts,
  onCancel,
  onSubmit,
}: OltPlacementModalProps) {
  const [form] = Form.useForm<OltPlacementValues>();

  const submit = () => {
    form
      .validateFields()
      .then((values) => onSubmit(values.oltId))
      // antd has already rendered the failure against its own field, so
      // there is nothing left to report — but without this the rejection
      // escapes as an unhandled promise.
      .catch(() => undefined);
  };

  return (
    <Modal
      open={open}
      title="Tempatkan OLT"
      onCancel={onCancel}
      onOk={submit}
      okText="Simpan"
      cancelText="Batal"
      destroyOnClose
    >
      <Descriptions column={1} size="small" style={{ marginBottom: 12 }}>
        <Descriptions.Item label="Koordinat">
          {position.lat.toFixed(6)}, {position.lng.toFixed(6)}
        </Descriptions.Item>
      </Descriptions>
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="oltId"
          label="OLT"
          rules={[{ required: true, message: "Pilih OLT" }]}
        >
          <Select
            placeholder="Pilih OLT yang belum ada di peta"
            options={olts.map((olt) => ({
              value: olt.id,
              label: `${olt.name} — ${olt.siteName}`,
            }))}
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
