import { Alert, Button, Form, Modal, message, type FormInstance } from "antd";
import type { ManagedSwitch, UpdateSwitchDto } from "@/domain/entities";
import {
  useCreateSwitch,
  useUpdateSwitch,
} from "@/application/hooks/useSwitches";
import { SwitchFormFields } from "./SwitchFormFields";
import { PROTOCOL_DEFAULT_PORT, protocolsFor } from "./switchProtocol";
import {
  errorText,
  useSwitchTest,
  type SwitchFormValues,
} from "./useSwitchTest";

const DEFAULT_SNMP_PORT = 161;

interface Props {
  sw?: ManagedSwitch;
  onClose: () => void;
}

function TestResult({ steps }: { steps: ReturnType<typeof useSwitchTest> }) {
  const { result, error } = steps;
  if (error) return <Alert type="error" message={error} />;
  if (!result) return null;
  if (result.kind === "identified") {
    return (
      <Alert
        type="success"
        message={`Terhubung: ${result.model} · ${result.version}`}
      />
    );
  }
  return (
    <Alert
      type="warning"
      message={`Fingerprint host key: ${result.presentedFingerprint}`}
      description="Cocokkan dengan host key SSH switch. Password baru dikirim setelah Anda mempercayai kunci ini."
      action={
        <Button
          size="small"
          loading={steps.testing}
          onClick={() => steps.trust(result.presentedFingerprint)}
        >
          Percayai kunci ini
        </Button>
      }
    />
  );
}

function initialValues(sw?: ManagedSwitch): Partial<SwitchFormValues> {
  const vendor = sw?.vendor ?? "huawei_ce";
  const protocol = sw?.protocol ?? protocolsFor(vendor)[0];
  return {
    vendor,
    name: sw?.name,
    host: sw?.host,
    snmpPort: sw?.snmpPort ?? DEFAULT_SNMP_PORT,
    protocol,
    port: sw?.port ?? PROTOCOL_DEFAULT_PORT[protocol],
    username: sw?.username,
    rackPosition: sw?.rackPosition ?? undefined,
  };
}

// The backend binding rejects a null rack_position against min=1, so an empty
// field is sent as an explicit clear; the vendor cannot change.
function updatePayload({
  rackPosition,
  vendor: _vendor,
  ...rest
}: SwitchFormValues): UpdateSwitchDto {
  return rackPosition == null
    ? { ...rest, clearRackPosition: true }
    : { ...rest, rackPosition };
}

function useSaveSwitch(
  form: FormInstance<SwitchFormValues>,
  sw: ManagedSwitch | undefined,
  hostKey: string | null,
  onClose: () => void,
) {
  const create = useCreateSwitch();
  const update = useUpdateSwitch();
  const save = async () => {
    try {
      const values = await form.validateFields();
      if (sw) {
        await update.mutateAsync({ id: sw.id, data: updatePayload(values) });
      } else if (hostKey !== null) {
        // A new switch has no rack field, so values carry nothing extra.
        await create.mutateAsync({
          ...values,
          hostKeyFingerprint: hostKey || undefined,
        });
      }
      onClose();
    } catch (error) {
      message.error(errorText(error));
    }
  };
  return { save, saving: create.isPending || update.isPending };
}

export function SwitchFormModal({ sw, onClose }: Props) {
  const [form] = Form.useForm<SwitchFormValues>();
  const steps = useSwitchTest(form);
  const editing = sw !== undefined;
  const { save, saving } = useSaveSwitch(form, sw, steps.savable, onClose);

  // A vendor change keeps a protocol it still speaks; a protocol change moves
  // the port to that protocol's default unless the operator typed one.
  const onValuesChange = (changed: Partial<SwitchFormValues>) => {
    if (changed.vendor) {
      const allowed = protocolsFor(changed.vendor);
      if (!allowed.includes(form.getFieldValue("protocol"))) {
        form.setFieldsValue({ protocol: allowed[0] });
        changed = { ...changed, protocol: allowed[0] };
      }
    }
    if (changed.protocol && !form.isFieldTouched("port")) {
      form.setFieldsValue({ port: PROTOCOL_DEFAULT_PORT[changed.protocol] });
    }
    if (!editing) steps.reset();
  };

  return (
    <Modal
      open
      title={editing ? "Ubah switch" : "Tambah switch"}
      onCancel={onClose}
      footer={[
        !editing && (
          <Button
            key="test"
            onClick={steps.run}
            loading={steps.testing && !steps.result}
          >
            Test koneksi
          </Button>
        ),
        <Button
          key="save"
          type="primary"
          disabled={!editing && steps.savable === null}
          loading={saving}
          onClick={save}
        >
          Simpan
        </Button>,
      ]}
    >
      <Form
        form={form}
        layout="vertical"
        onValuesChange={onValuesChange}
        initialValues={initialValues(sw)}
      >
        <SwitchFormFields editing={editing} />
      </Form>
      <TestResult steps={steps} />
    </Modal>
  );
}
