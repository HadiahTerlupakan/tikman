import { Form, Input, Modal, Select } from "antd";
import { useEffect } from "react";
import type { FiberType, MappingEdge } from "@/domain/entities";
import { FIBER_OPTIONS } from "./mappingLabels";

interface EdgeFormModalProps {
  open: boolean;
  /** The cable being edited. Source, target and waypoints are geometry —
   * they come from this and are never offered as fields here; changing
   * them belongs to redrawing the cable on the map, not a table-row edit. */
  initial: MappingEdge;
  onCancel: () => void;
  onSubmit: (edge: MappingEdge) => void;
}

interface EdgeFormValues {
  fiberType: FiberType;
  notes?: string;
}

export function EdgeFormModal({
  open,
  initial,
  onCancel,
  onSubmit,
}: EdgeFormModalProps) {
  const [form] = Form.useForm<EdgeFormValues>();

  useEffect(() => {
    form.setFieldsValue({ fiberType: initial.fiberType, notes: initial.notes });
  }, [form, initial]);

  const submit = () => {
    form
      .validateFields()
      .then((values) => {
        onSubmit({
          ...initial,
          fiberType: values.fiberType,
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
      title="Ubah kabel"
      onCancel={onCancel}
      onOk={submit}
      okText="Simpan"
      cancelText="Batal"
      destroyOnClose
    >
      <Form form={form} layout="vertical" preserve={false}>
        <Form.Item name="fiberType" label="Jenis kabel">
          <Select<FiberType>
            style={{ width: "100%" }}
            options={FIBER_OPTIONS}
          />
        </Form.Item>
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
