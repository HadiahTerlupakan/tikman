import {
  Alert,
  App,
  Form,
  Modal,
  Spin,
  Switch,
  Typography,
  type FormInstance,
} from "antd";
import {
  usePortAlertDestination,
  useSavePortAlertDestination,
} from "@/application/hooks";
import type { PortAlertDestination } from "@/domain/entities";
import { PortAlertDestinationFields } from "./PortAlertDestinationFields";
import {
  destinationToFormValues,
  toPortAlertDestinationInput,
  type PortAlertFormValues,
} from "./portAlertDestination";
import { GENERAL_TOPIC } from "./telegramDestination";

const WEBHOOK_URL = "https://noc.radpro.id/api/v1/hooks/librenms";
const WEBHOOK_BODY = '{"device_id":"{{ $device_id }}"}';

// LibreNMS only says which device raised an alert; TikMan then reads that
// device's ports itself, so the body carries nothing but the device id.
function LibrenmsSetupHelp() {
  const { Text } = Typography;
  return (
    <Alert
      type="info"
      style={{ marginBottom: 16 }}
      message="Pengaturan di LibreNMS"
      description={
        <ol style={{ paddingLeft: 20, margin: 0 }}>
          <li>
            Buat Alert Transport tipe <Text code>API</Text>, metode{" "}
            <Text code>POST</Text>, URL <Text code>{WEBHOOK_URL}</Text>.
          </li>
          <li>
            Header <Text code>X-TikMan-Token=&lt;token&gt;</Text>, token sama
            dengan "LibreNMS webhook token" di Pengaturan.
          </li>
          <li>
            Body <Text code>{WEBHOOK_BODY}</Text>.
          </li>
          <li>
            Pasang transport itu pada rule "Port status up/down" dan aktifkan
            recovery alert di rule tersebut, agar notifikasi UP ikut terkirim.
          </li>
        </ol>
      }
    />
  );
}

interface FormProps {
  saved: PortAlertDestination;
  form: FormInstance<PortAlertFormValues>;
}

function PortAlertForm({ saved, form }: FormProps) {
  // A group row belongs to one number, and a topic to one chat; General is
  // always open to the NOC, so a new chat starts there.
  const handleValuesChange = (changed: Partial<PortAlertFormValues>) => {
    if ("waAccountId" in changed) form.setFieldValue("waGroupId", undefined);
    if ("telegramChatId" in changed)
      form.setFieldValue("telegramThreadId", GENERAL_TOPIC);
  };
  return (
    // The page keeps this modal mounted; clearOnDestroy stops the last
    // edit's unsaved values from merging over the saved ones on reopen.
    <Form
      form={form}
      clearOnDestroy
      layout="vertical"
      initialValues={destinationToFormValues(saved)}
      onValuesChange={handleValuesChange}
    >
      <PortAlertDestinationFields form={form} saved={saved} />
      <Form.Item name="enabled" label="Aktif" valuePropName="checked">
        <Switch />
      </Form.Item>
    </Form>
  );
}

interface PortAlertDestinationModalProps {
  open: boolean;
  onClose: () => void;
}

/** The NOC's destination for port down/up notices of every report. */
export function PortAlertDestinationModal({
  open,
  onClose,
}: PortAlertDestinationModalProps) {
  const { message } = App.useApp();
  const [form] = Form.useForm<PortAlertFormValues>();
  const { data, isLoading, error } = usePortAlertDestination(open);
  const save = useSavePortAlertDestination();

  const handleOk = () => {
    form
      .validateFields()
      .then((values) =>
        save.mutate(toPortAlertDestinationInput(values), {
          onSuccess: () => {
            message.success("Tujuan notifikasi NOC disimpan");
            onClose();
          },
          onError: (err) => message.error(err.message),
        }),
      )
      // antd has already marked each failed field.
      .catch(() => undefined);
  };

  return (
    <Modal
      open={open}
      title="Notifikasi NOC"
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={save.isPending}
      okButtonProps={{ disabled: !data }}
      onOk={handleOk}
      onCancel={onClose}
      destroyOnHidden
    >
      <Typography.Paragraph type="secondary">
        Setiap port laporan yang down atau kembali up dikirim ke sini, termasuk
        laporan yang tidak mengirim notifikasi ke pelanggannya.
      </Typography.Paragraph>
      <LibrenmsSetupHelp />
      {error && <Alert type="error" showIcon message={error.message} />}
      {isLoading && <Spin />}
      {data && <PortAlertForm saved={data} form={form} />}
    </Modal>
  );
}
