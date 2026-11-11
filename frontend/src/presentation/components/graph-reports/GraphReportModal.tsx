import { useState } from "react";
import {
  Alert,
  Button,
  Checkbox,
  Form,
  Input,
  Modal,
  Switch,
  TimePicker,
} from "antd";
import { FileImageOutlined } from "@ant-design/icons";
import {
  useGraphReportPreview,
  useLibrenmsDevices,
  useTelegramChats,
} from "@/application/hooks";
import {
  DEFAULT_GRAPH_REPORT_MESSAGE,
  GRAPH_REPORT_MESSAGE_MAX,
  type GraphReport,
  type GraphReportInput,
} from "@/domain/entities";
import { DestinationFields } from "./DestinationFields";
import {
  emptyFormValues,
  reportToFormValues,
  toGraphReportInput,
  toPreviewInput,
  type GraphReportFormValues,
} from "./graphReportForm";
import { initialTopic } from "./telegramDestination";
import { ImagePreviewModal } from "./ImagePreviewModal";
import { PortList } from "./PortList";
import { PortPreviewModal, type PreviewTarget } from "./PortPreviewModal";

// The destination is not part of the image or caption, so a preview does not
// wait for one to be chosen.
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
  const devices = useLibrenmsDevices(open);
  const { data: telegramChats = [] } = useTelegramChats(open);
  const waGroupId = Form.useWatch("waGroupId", form);

  // A group row belongs to one number, so the old choice is never valid for
  // the new one. Tagging everyone is WhatsApp's alone, so clearing the group
  // unticks it; switching number does not, since the admin is about to pick
  // the same customer's group on the new number, and toGraphReportInput
  // already sends no tag while no group is chosen.
  const handleValuesChange = (changed: Partial<GraphReportFormValues>) => {
    if ("waAccountId" in changed) form.setFieldValue("waGroupId", undefined);
    // A topic belongs to one forum.
    if ("telegramChatId" in changed) {
      const chat = telegramChats.find((c) => c.id === changed.telegramChatId);
      form.setFieldValue(
        "telegramThreadId",
        initialTopic(chat, reports, { report }),
      );
    }
    if ("waGroupId" in changed && !changed.waGroupId)
      form.setFieldValue("mentionAll", false);
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
        <DestinationFields
          form={form}
          open={open}
          report={report}
          reports={reports}
        />
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
          extra={
            waGroupId
              ? "Semua anggota mendapat notifikasi, termasuk yang membisukan grup."
              : "Hanya untuk grup WhatsApp; Telegram tidak punya tag semua."
          }
        >
          <Checkbox disabled={!waGroupId}>Tag semua anggota grup</Checkbox>
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
