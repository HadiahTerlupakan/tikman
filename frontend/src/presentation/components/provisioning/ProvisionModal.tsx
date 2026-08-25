import { useState } from "react";
import { Modal, Form, Select, Switch, Input, message, Button } from "antd";
import type { ConfigTemplate } from "@/domain/entities/ConfigTemplate";
import type { ProvisionRequest } from "@/domain/entities/Provisioning";

function parseManualConfig(value: string | undefined): Record<string, unknown> {
  if (!value?.trim()) return {};
  try {
    const parsed: unknown = JSON.parse(value);
    if (parsed && typeof parsed === "object" && !Array.isArray(parsed)) {
      return parsed as Record<string, unknown>;
    }
  } catch {
    message.error("Konfigurasi manual harus berupa JSON object yang valid");
  }
  return {};
}

interface ProvisionModalProps {
  open: boolean;
  ontId?: string;
  templates?: ConfigTemplate[];
  onClose: () => void;
  onSubmit: (data: ProvisionRequest) => void;
  loading: boolean;
}

export function ProvisionModal({
  open,
  ontId,
  templates,
  onClose,
  onSubmit,
  loading,
}: ProvisionModalProps) {
  const [form] = Form.useForm();
  const [isConfirmed, setIsConfirmed] = useState(false);

  const handleConfirmChange = (checked: boolean) => {
    if (!ontId) {
      message.warning("Pilih ONT terlebih dahulu untuk provisioning");
      return;
    }
    setIsConfirmed(checked);
  };

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit({
        templateId: values.templateId || undefined,
        manualConfig: parseManualConfig(values.manualConfig),
        confirm: isConfirmed,
      });
    });
  };

  return (
    <Modal
      title="Single ONT Provisioning"
      open={open && !!ontId}
      onOk={handleSubmit}
      onCancel={() => {
        setIsConfirmed(false);
        onClose();
      }}
      confirmLoading={loading}
      footer={[
        <Switch
          key="confirm-switch"
          checked={isConfirmed}
          onChange={handleConfirmChange}
          style={{ marginRight: 16 }}
          size="small"
        />,
        <span key="confirm-text">Saya sudah memeriksa konfigurasi</span>,
        <Button key="cancel" onClick={onClose}>
          Batal
        </Button>,
        <Button
          key="submit"
          type="primary"
          loading={loading}
          disabled={!isConfirmed}
          onClick={handleSubmit}
        >
          Kirim
        </Button>,
      ]}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item name="templateId" label="Configuration Template">
          <Select
            showSearch
            placeholder="Pilih template atau biarkan kosong untuk manual config"
            allowClear
            optionFilterProp="children"
            options={
              templates?.map((t) => ({
                value: t.id,
                label: `${t.name} (${t.vendor})`,
              })) || []
            }
          />
        </Form.Item>

        <Form.Item
          name="manualConfig"
          label="Manual Configuration Fields"
          tooltip="Bidang tambahan dapat ditambahkan di sini saat implementasi lanjutan"
        >
          <Input.TextArea
            rows={4}
            disabled={!ontId}
            placeholder="Konfigurasi manual akan ditambahkan disini"
          />
        </Form.Item>
      </Form>
    </Modal>
  );
}
