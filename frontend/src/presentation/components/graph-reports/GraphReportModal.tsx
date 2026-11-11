import { useEffect, useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Select,
  Switch,
  TimePicker,
} from "antd";
import { FileImageOutlined } from "@ant-design/icons";
import {
  useGraphReportPreview,
  useLibrenmsDevices,
  useWaAccounts,
  useWaGroups,
} from "@/application/hooks";
import {
  DEFAULT_GRAPH_REPORT_MESSAGE,
  GRAPH_REPORT_MESSAGE_MAX,
  type GraphReport,
  type GraphReportInput,
} from "@/domain/entities";
import {
  accountOptions,
  emptyFormValues,
  groupOptions,
  initialAccountId,
  reportToFormValues,
  toGraphReportInput,
  toPreviewInput,
  type GraphReportFormValues,
} from "./graphReportForm";
import { ImagePreviewModal } from "./ImagePreviewModal";
import { PortList } from "./PortList";
import { PortPreviewModal, type PreviewTarget } from "./PortPreviewModal";

// The group is not part of the image or caption, so a preview does not wait
// for one to be chosen.
const PREVIEW_FIELDS = ["customerName", "message", "ports"];

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
  const [previewOpen, setPreviewOpen] = useState(false);
  const reportPreview = useGraphReportPreview();
  const { data: accounts = [], isLoading: accountsLoading } = useWaAccounts();
  const devices = useLibrenmsDevices(open);
  const waAccountId = Form.useWatch("waAccountId", form);
  // The unfiltered list is capped across every number, so a quiet number's
  // groups could fall off it; asking for the chosen number's own is not.
  const { data: groups = [], isLoading: groupsLoading } = useWaGroups(
    waAccountId ? { waAccountId } : undefined,
  );
  const options = groupOptions(groups, reports, {
    waAccountId,
    currentGroupId: report?.waGroupId,
  });

  // A report listed without its number gets it from its group, which may
  // load after the form has mounted.
  useEffect(() => {
    if (!report || form.getFieldValue("waAccountId")) return;
    const id = initialAccountId(report, groups);
    if (id) form.setFieldValue("waAccountId", id);
  }, [report, groups, form]);

  // A group row belongs to one number, so the old choice is never valid for
  // the new one.
  const handleValuesChange = (changed: Partial<GraphReportFormValues>) => {
    if ("waAccountId" in changed) form.setFieldValue("waGroupId", undefined);
  };

  const handleOk = () => {
    form
      .validateFields()
      .then((values) => onSubmit(toGraphReportInput(values)))
      // antd has already marked each failed field.
      .catch(() => undefined);
  };

  const handlePreview = () => {
    form
      .validateFields(PREVIEW_FIELDS, { recursive: true })
      .then((values) => {
        const input = toPreviewInput(values);
        setPreviewOpen(true);
        reportPreview.mutate(input);
      })
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
        onValuesChange={handleValuesChange}
      >
        <Form.Item
          name="waAccountId"
          label="Nomor WhatsApp"
          extra="Laporan dikirim dari nomor ini."
          rules={[{ required: true, message: "Pilih nomor" }]}
        >
          <Select
            placeholder="Pilih nomor"
            loading={accountsLoading}
            options={accountOptions(accounts)}
          />
        </Form.Item>
        <Form.Item
          name="waGroupId"
          label="Grup WhatsApp"
          rules={[{ required: true, message: "Pilih grup" }]}
        >
          <Select
            showSearch
            placeholder={waAccountId ? "Pilih grup" : "Pilih nomor dahulu"}
            optionFilterProp="label"
            disabled={!waAccountId}
            loading={groupsLoading}
            options={options}
            notFoundContent={
              waAccountId && !groupsLoading
                ? "Nomor ini belum punya grup yang bisa dikirimi"
                : undefined
            }
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
        <Form.Item
          name="message"
          label="Pesan"
          extra="Kosongkan untuk memakai pesan bawaan. {pelanggan} diganti nama pelanggan."
          rules={[{ max: GRAPH_REPORT_MESSAGE_MAX }]}
        >
          <Input.TextArea
            placeholder={DEFAULT_GRAPH_REPORT_MESSAGE}
            autoSize={{ minRows: 4, maxRows: 10 }}
            maxLength={GRAPH_REPORT_MESSAGE_MAX}
            showCount
          />
        </Form.Item>
        <Form.Item
          name="sendTime"
          label="Jam kirim"
          extra="Waktu WIB. Jika gagal, dicoba ulang tiap 10 menit selama 3 jam (maksimal sampai 24:00)."
          rules={[{ required: true, message: "Pilih jam kirim" }]}
        >
          <TimePicker
            format="HH:mm"
            minuteStep={5}
            needConfirm={false}
            allowClear={false}
          />
        </Form.Item>
        <Form.Item name="enabled" label="Aktif" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item
          name="mentionAll"
          valuePropName="checked"
          extra="Semua anggota mendapat notifikasi, termasuk yang membisukan grup."
        >
          <Checkbox>Tag semua anggota grup</Checkbox>
        </Form.Item>
        <PortList
          devices={devices.data ?? []}
          devicesLoading={devices.isLoading}
          onPreview={setPreview}
        />
      </Form>
      <Button
        icon={<FileImageOutlined />}
        style={{ marginTop: 16 }}
        onClick={handlePreview}
      >
        Pratinjau laporan
      </Button>
      <PortPreviewModal target={preview} onClose={() => setPreview(null)} />
      <ImagePreviewModal
        open={previewOpen}
        title="Pratinjau laporan"
        alt="Grafik trafik laporan"
        images={reportPreview.data}
        loading={reportPreview.isPending}
        error={reportPreview.error}
        onClose={() => setPreviewOpen(false)}
      />
    </Modal>
  );
}
