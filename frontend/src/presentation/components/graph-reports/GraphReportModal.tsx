import { useState } from "react";
import { Alert, Form, Input, Modal, Select, Switch } from "antd";
import { useLibrenmsDevices, useWaGroups } from "@/application/hooks";
import type { GraphReport, GraphReportInput } from "@/domain/entities";
import {
  emptyFormValues,
  groupOptions,
  reportToFormValues,
  toGraphReportInput,
  type GraphReportFormValues,
} from "./graphReportForm";
import { PortList } from "./PortList";
import { PortPreviewModal, type PreviewTarget } from "./PortPreviewModal";

interface GraphReportModalProps {
  open: boolean;
  report?: GraphReport;
  reports: GraphReport[];
  loading: boolean;
  onClose: () => void;
  onSubmit: (input: GraphReportInput) => void;
}

export function GraphReportModal({
  open,
  report,
  reports,
  loading,
  onClose,
  onSubmit,
}: GraphReportModalProps) {
  const [form] = Form.useForm<GraphReportFormValues>();
  const [preview, setPreview] = useState<PreviewTarget | null>(null);
  const { data: groups = [], isLoading: groupsLoading } = useWaGroups();
  const devices = useLibrenmsDevices(open);

  const handleOk = () => {
    form
      .validateFields()
      .then((values) => onSubmit(toGraphReportInput(values)))
      // antd has already marked each failed field.
      .catch(() => undefined);
  };

  return (
    <Modal
      open={open}
      title={report ? "Ubah Laporan Grafik" : "Tambah Laporan Grafik"}
      okText="Simpan"
      cancelText="Batal"
      width={900}
      confirmLoading={loading}
      onOk={handleOk}
      onCancel={onClose}
      destroyOnHidden
    >
      {devices.error && (
        <Alert
          type="error"
          showIcon
          message={devices.error.message}
          style={{ marginBottom: 16 }}
        />
      )}
      {/* destroyOnHidden remounts the form on every open, so initialValues
          are what each open starts from. */}
      <Form
        form={form}
        layout="vertical"
        initialValues={report ? reportToFormValues(report) : emptyFormValues()}
      >
        <Form.Item
          name="waGroupId"
          label="Grup WhatsApp"
          rules={[{ required: true, message: "Pilih grup" }]}
        >
          <Select
            showSearch
            placeholder="Pilih grup"
            optionFilterProp="label"
            loading={groupsLoading}
            options={groupOptions(groups, reports, report?.waGroupId)}
          />
        </Form.Item>
        <Form.Item
          name="customerName"
          label="Nama pelanggan"
          rules={[
            { required: true, whitespace: true, message: "Isi nama pelanggan" },
            { max: 255 },
          ]}
        >
          <Input />
        </Form.Item>
        <Form.Item name="enabled" label="Aktif" valuePropName="checked">
          <Switch />
        </Form.Item>
        <PortList
          devices={devices.data ?? []}
          devicesLoading={devices.isLoading}
          onPreview={setPreview}
        />
      </Form>
      <PortPreviewModal target={preview} onClose={() => setPreview(null)} />
    </Modal>
  );
}
