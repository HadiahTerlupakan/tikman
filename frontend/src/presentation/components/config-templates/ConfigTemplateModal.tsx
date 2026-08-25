import { Modal, Form, Input, Select, Switch } from "antd";
import { useEffect } from "react";
import type {
  ConfigTemplate,
  CreateConfigTemplateDto,
  UpdateConfigTemplateDto,
} from "@/domain/entities/ConfigTemplate";

interface ConfigTemplateModalProps {
  open: boolean;
  template?: ConfigTemplate;
  onClose: () => void;
  onSubmit: (data: CreateConfigTemplateDto | UpdateConfigTemplateDto) => void;
  loading: boolean;
}

const VENDOR_OPTIONS = [
  { value: "ZTE", label: "ZTE" },
  { value: "HSGQ", label: "HSGQ" },
];

export function ConfigTemplateModal({
  open,
  template,
  onClose,
  onSubmit,
  loading,
}: ConfigTemplateModalProps) {
  const [form] = Form.useForm();

  useEffect(() => {
    if (template) {
      form.setFieldsValue({
        name: template.name,
        description: template.description,
        vendor: template.vendor,
        isDefault: template.isDefault,
      });
    } else {
      form.resetFields();
    }
  }, [template, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      const payload = template
        ? ({
            ...values,
            configFields: template.configFields,
          } as UpdateConfigTemplateDto)
        : ({ ...values, configFields: {} } as CreateConfigTemplateDto);
      onSubmit(payload);
    });
  };

  return (
    <Modal
      title={template ? "Edit Config Template" : "Create Config Template"}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Template Name"
          rules={[
            { required: true, message: "Please enter template name" },
            { min: 3, message: "Name must be at least 3 characters" },
          ]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="vendor"
          label="Vendor"
          rules={[{ required: true, message: "Please select vendor" }]}
        >
          <Select options={VENDOR_OPTIONS} />
        </Form.Item>

        <Form.Item name="description" label="Description">
          <Input.TextArea rows={3} />
        </Form.Item>

        <Form.Item
          name="isDefault"
          label="Default Template"
          valuePropName="checked"
          tooltip="Menjadikan template ini default untuk vendor terkait"
        >
          <Switch />
        </Form.Item>
      </Form>
    </Modal>
  );
}
