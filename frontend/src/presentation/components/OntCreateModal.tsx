import { Modal, Form, Select, InputNumber, Input } from "antd";
import type { CreateOntDto } from "@/domain/entities";

const { Option } = Select;

interface OntCreateModalProps {
  open: boolean;
  onCancel: () => void;
  onSubmit: (values: CreateOntDto) => Promise<void>;
  form: any;
  oltsData: Array<{ id: string; name: string }>;
  isLoading: boolean;
}

export function OntCreateModal({
  open,
  onCancel,
  onSubmit,
  form,
  oltsData,
  isLoading,
}: OntCreateModalProps) {
  return (
    <Modal
      title="Add New ONT"
      open={open}
      onCancel={onCancel}
      onOk={() => form.submit()}
      confirmLoading={isLoading}
    >
      <Form form={form} layout="vertical" onFinish={onSubmit}>
        <Form.Item
          name="oltId"
          label="OLT"
          rules={[{ required: true, message: "Please select an OLT" }]}
        >
          <Select placeholder="Select OLT">
            {oltsData?.map((olt) => (
              <Option key={olt.id} value={olt.id} label={olt.name}>
                {olt.name}
              </Option>
            ))}
          </Select>
        </Form.Item>
        <Form.Item
          name="portId"
          label="Port ID"
          rules={[
            { required: true, message: "Please enter port ID" },
            { type: "number", min: 0, max: 15, message: "Port ID must be 0-15" },
          ]}
        >
          <InputNumber style={{ width: "100%" }} min={0} max={15} />
        </Form.Item>
        <Form.Item
          name="ontId"
          label="ONT ID"
          rules={[
            { required: true, message: "Please enter ONT ID" },
            { type: "number", min: 0, max: 127, message: "ONT ID must be 0-127" },
          ]}
        >
          <InputNumber style={{ width: "100%" }} min={0} max={127} />
        </Form.Item>
        <Form.Item
          name="serialNumber"
          label="Serial Number"
          rules={[
            { required: true, message: "Please enter serial number" },
            { max: 20, message: "Serial number max 20 characters" },
          ]}
        >
          <Input placeholder="e.g., ZTEG12345678" />
        </Form.Item>
        <Form.Item name="description" label="Description">
          <Input.TextArea rows={3} placeholder="Optional description" />
        </Form.Item>
      </Form>
    </Modal>
  );
}
