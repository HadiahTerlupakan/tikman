import { Checkbox, Form, Input, InputNumber, Modal } from "antd";
import { useEffect } from "react";
import type { MappingNode, NodeType, Waypoint } from "@/domain/entities";
import { parseCoordinate } from "@/presentation/components/sites/siteCoordinates";

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
  manualCoords: boolean;
  capacity?: number;
  splitter?: string;
  pppoe?: string;
  serialNumber?: string;
  notes?: string;
}

const LATITUDE_RANGE = { min: -90, max: 90 };
const LONGITUDE_RANGE = { min: -180, max: 180 };

// A new node starts blank except for where the map was tapped; editing shows
// the node's own stored fields instead, since re-tapping the map is not how
// an existing box gets opened. The checkbox always starts unticked either
// way — the fields stay read-only until the operator explicitly asks to
// override the map tap or the stored position.
function initialValues(
  initial: MappingNode | undefined,
  position: Waypoint,
): NodeFormValues {
  if (!initial) {
    return {
      name: "",
      latitude: String(position.lat),
      longitude: String(position.lng),
      manualCoords: false,
    };
  }
  return {
    name: initial.name,
    nodeId: initial.nodeId,
    latitude: String(initial.latitude),
    longitude: String(initial.longitude),
    manualCoords: false,
    capacity: initial.capacity,
    splitter: initial.splitter,
    pppoe: initial.pppoe,
    serialNumber: initial.serialNumber,
    notes: initial.notes,
  };
}

// A coordinate typed by hand is the only path this form has besides a map tap
// or the stored record, neither of which can be out of range — it is the one
// case that actually needs checking. parseCoordinate is shared with the
// site/OLT location fields so a comma decimal ("6,21") is rejected the same
// way everywhere, instead of silently parsing as a different place.
function coordinateRule(range: { min: number; max: number }, label: string) {
  return {
    validator: (_: unknown, value: string) => {
      const parsed = parseCoordinate(value ?? "");
      if (parsed === null || parsed < range.min || parsed > range.max) {
        return Promise.reject(
          new Error(`${label} harus di antara ${range.min} dan ${range.max}`),
        );
      }
      return Promise.resolve();
    },
  };
}

/** Latitude/longitude plus the checkbox that unlocks them. Split out of
 * NodeFormModal only to keep that component's render under the file's line
 * limit — there is still exactly one caller. */
function CoordinateFields({ manual }: { manual: boolean }) {
  return (
    <>
      <Form.Item name="manualCoords" valuePropName="checked">
        <Checkbox>Isi koordinat manual</Checkbox>
      </Form.Item>
      <Form.Item
        name="latitude"
        label="Latitude"
        rules={[coordinateRule(LATITUDE_RANGE, "Latitude")]}
      >
        <Input readOnly={!manual} />
      </Form.Item>
      <Form.Item
        name="longitude"
        label="Longitude"
        rules={[coordinateRule(LONGITUDE_RANGE, "Longitude")]}
      >
        <Input readOnly={!manual} />
      </Form.Item>
    </>
  );
}

// capacity/splitter only mount for a non-ONT type, pppoe/serialNumber only
// for an ONT (see effectiveType below) — the field that does not mount for
// this node's type has no form value to read, so it has to fall back to
// whatever the record already had instead of the submit's own `?? 0` / `?? ""`,
// or an edit that never touched that field would silently zero it out.
function buildNodeFromValues(
  values: NodeFormValues,
  initial: MappingNode | undefined,
  effectiveType: NodeType,
): MappingNode {
  const nodeId = initial
    ? initial.nodeId
    : values.nodeId || `${effectiveType.toUpperCase()}-${Date.now()}`;
  const isOnt = effectiveType === "ont";

  return {
    id: initial?.id,
    nodeId,
    type: effectiveType,
    name: values.name,
    latitude: Number(values.latitude),
    longitude: Number(values.longitude),
    capacity: isOnt ? initial?.capacity ?? 0 : values.capacity ?? 0,
    splitter: isOnt ? initial?.splitter ?? "" : values.splitter ?? "",
    pppoe: isOnt ? values.pppoe ?? "" : initial?.pppoe ?? "",
    serialNumber: isOnt
      ? values.serialNumber ?? ""
      : initial?.serialNumber ?? "",
    notes: values.notes ?? "",
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
  const manualCoords = Form.useWatch<boolean | undefined>("manualCoords", form);

  useEffect(() => {
    form.setFieldsValue(initialValues(initial, position));
  }, [form, initial, position]);

  const submit = () => {
    form
      .validateFields()
      .then((values) =>
        onSubmit(buildNodeFromValues(values, initial, effectiveType)),
      )
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
        <CoordinateFields manual={Boolean(manualCoords)} />
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
