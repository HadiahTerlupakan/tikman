import { Form, Input, InputNumber, Modal } from "antd";
import { useEffect } from "react";
import type { MappingNode, NodeType, Waypoint } from "@/domain/entities";

interface NodeFormModalProps {
  open: boolean;
  type: NodeType;
  position: Waypoint;
  /** The node being edited. Absent, the form places a new one instead. */
  initial?: MappingNode;
  onCancel: () => void;
  onSubmit: (node: MappingNode) => void;
}

interface NodeFormValues {
  name: string;
  nodeId?: string;
  latitude: string;
  longitude: string;
  capacity?: number;
  splitter?: string;
  pppoe?: string;
  serialNumber?: string;
  notes?: string;
}

// A new node starts blank except for where the map was tapped; editing shows
// the node's own stored fields instead, since re-tapping the map is not how
// an existing box gets opened.
function initialValues(
  initial: MappingNode | undefined,
  position: Waypoint,
): NodeFormValues {
  if (!initial) {
    return {
      name: "",
      latitude: String(position.lat),
      longitude: String(position.lng),
    };
  }
  return {
    name: initial.name,
    nodeId: initial.nodeId,
    latitude: String(initial.latitude),
    longitude: String(initial.longitude),
    capacity: initial.capacity,
    splitter: initial.splitter,
    pppoe: initial.pppoe,
    serialNumber: initial.serialNumber,
    notes: initial.notes,
  };
}

export function NodeFormModal({
  open,
  type,
  position,
  initial,
  onCancel,
  onSubmit,
}: NodeFormModalProps) {
  const [form] = Form.useForm<NodeFormValues>();
  // `initial` already wins over every prop derived from the map/toolbar state
  // for name and position; `type` must follow the same rule. Trusting the
  // placement `type` for an existing node of a different type doesn't just
  // mislabel it — it gates the wrong Form.Items into existence, so the node's
  // real capacity/splitter (or PPPoE/serial) never mount and are lost on save.
  const effectiveType = initial?.type ?? type;

  useEffect(() => {
    form.setFieldsValue(initialValues(initial, position));
  }, [form, initial, position]);

  const submit = () => {
    form
      .validateFields()
      .then((values) => {
        // Editing never trusts the field for the id: it is read-only, but
        // reading an empty value out of it by accident would mint a fresh one
        // and orphan every cable that points at the old id.
        const nodeId = initial
          ? initial.nodeId
          : values.nodeId || `${effectiveType.toUpperCase()}-${Date.now()}`;

        onSubmit({
          id: initial?.id,
          nodeId,
          type: effectiveType,
          name: values.name,
          latitude: Number(values.latitude),
          longitude: Number(values.longitude),
          capacity: values.capacity ?? 0,
          splitter: values.splitter ?? "",
          pppoe: values.pppoe ?? "",
          serialNumber: values.serialNumber ?? "",
          notes: values.notes ?? "",
        });
      })
      // antd has already rendered the failure against its own field, so there
      // is nothing left to report — but without this the rejection escapes as
      // an unhandled promise.
      .catch(() => undefined);
  };

  return (
    <Modal
      open={open}
      title={initial ? "Ubah node" : "Tambah node"}
      onCancel={onCancel}
      onOk={submit}
      okText="Simpan"
      cancelText="Batal"
      destroyOnClose
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item
          name="name"
          label="Nama"
          rules={[{ required: true, message: "Nama harus diisi" }]}
        >
          <Input placeholder="mis. ODP Depan Masjid" />
        </Form.Item>
        <Form.Item name="nodeId" label="Kode">
          <Input
            readOnly={Boolean(initial)}
            placeholder={
              initial ? undefined : "dibuat otomatis bila dikosongkan"
            }
          />
        </Form.Item>
        {effectiveType !== "ont" && (
          <>
            <Form.Item name="capacity" label="Jumlah slot">
              <InputNumber min={0} style={{ width: "100%" }} />
            </Form.Item>
            <Form.Item name="splitter" label="Rasio splitter">
              <Input placeholder="mis. 1:8" />
            </Form.Item>
          </>
        )}
        {effectiveType === "ont" && (
          <>
            <Form.Item name="pppoe" label="PPPoE">
              <Input />
            </Form.Item>
            <Form.Item name="serialNumber" label="Serial">
              <Input />
            </Form.Item>
          </>
        )}
        <Form.Item name="latitude" label="Latitude">
          <Input readOnly />
        </Form.Item>
        <Form.Item name="longitude" label="Longitude">
          <Input readOnly />
        </Form.Item>
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
