import {
  Alert,
  Button,
  Form,
  Modal,
  Space,
  message,
  type FormInstance,
} from "antd";
import type {
  MikrotikRouter,
  RouterApiProtocol,
  RouterConnectionTestResult,
} from "@/domain/entities";
import {
  useCreateMikrotikRouter,
  useUpdateMikrotikRouter,
} from "@/application/hooks/useMikrotikRouters";
import { RouterFormFields } from "./RouterFormFields";
import {
  PROTOCOL_DEFAULT_PORT,
  PROTOCOL_LABEL,
  usesTls,
} from "./routerProtocol";
import { requestErrorText } from "./routerTestSteps";
import {
  useRouterTestSteps,
  type RouterFormValues,
} from "./useRouterTestSteps";

const DEFAULT_SNMP_PORT = 161;

interface Props {
  router?: MikrotikRouter;
  onClose: () => void;
}

function snmpAlert(result: RouterConnectionTestResult) {
  return (
    <Alert
      type={result.snmpError ? "error" : "success"}
      message={result.snmpError ? `SNMP: ${result.snmpError}` : "SNMP menjawab"}
    />
  );
}

// Step 1 over TLS only reads the certificate: it must not claim the API worked.
function FingerprintStep({
  result,
  label,
  onConfirm,
  confirming,
}: {
  result: RouterConnectionTestResult;
  label: string;
  onConfirm: () => void;
  confirming: boolean;
}) {
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {snmpAlert(result)}
      {result.fingerprint ? (
        <Alert
          type="warning"
          message={`Fingerprint sertifikat: ${result.fingerprint}`}
          description={`Cocokkan dengan hasil \`/certificate print detail\` di router. Password ${label} baru dikirim setelah Anda mengonfirmasi.`}
          action={
            <Button size="small" loading={confirming} onClick={onConfirm}>
              Fingerprint cocok, uji {label}
            </Button>
          }
        />
      ) : (
        <Alert
          type="error"
          message={`${label}: ${result.apiError ?? "sertifikat tidak terbaca"}`}
        />
      )}
    </Space>
  );
}

function ApiStep({
  result,
  label,
}: {
  result: RouterConnectionTestResult;
  label: string;
}) {
  return (
    <Alert
      type={result.apiError ? "error" : "success"}
      message={
        result.apiError
          ? `${label}: ${result.apiError}`
          : `${label}: ${result.boardName} · RouterOS ${result.version}`
      }
    />
  );
}

function TestStepResults({
  steps,
  protocol,
}: {
  steps: ReturnType<typeof useRouterTestSteps>;
  protocol: RouterApiProtocol;
}) {
  const label = PROTOCOL_LABEL[protocol];
  const { first, second } = steps;
  if (!first) return null;
  if (!usesTls(protocol)) {
    return (
      <Space direction="vertical" style={{ width: "100%" }}>
        {snmpAlert(first)}
        <ApiStep result={first} label={label} />
      </Space>
    );
  }
  if (!second) {
    return (
      <FingerprintStep
        result={first}
        label={label}
        onConfirm={steps.runSecondStep}
        confirming={steps.testing}
      />
    );
  }
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {snmpAlert(second)}
      <ApiStep result={second} label={label} />
    </Space>
  );
}

function initialValues(router?: MikrotikRouter): Partial<RouterFormValues> {
  const apiProtocol = router?.apiProtocol ?? "rest";
  return {
    name: router?.name,
    host: router?.host,
    snmpPort: router?.snmpPort ?? DEFAULT_SNMP_PORT,
    apiUsername: router?.apiUsername,
    apiProtocol,
    apiPort: router?.apiPort ?? PROTOCOL_DEFAULT_PORT[apiProtocol],
  };
}

// A new router is saved only once the test passed; "" means plain API, which
// has no fingerprint to send.
function useSaveRouter(
  form: FormInstance<RouterFormValues>,
  router: MikrotikRouter | undefined,
  fingerprint: string | null,
  onClose: () => void,
) {
  const create = useCreateMikrotikRouter();
  const update = useUpdateMikrotikRouter();
  const save = async () => {
    try {
      const values = await form.validateFields();
      if (router) {
        await update.mutateAsync({ id: router.id, data: values });
      } else if (fingerprint !== null) {
        await create.mutateAsync({
          ...values,
          tlsFingerprint: fingerprint || undefined,
        });
      }
      onClose();
    } catch (error) {
      message.error(requestErrorText(error));
    }
  };
  return { save, saving: create.isPending || update.isPending };
}

export function RouterFormModal({ router, onClose }: Props) {
  const [form] = Form.useForm<RouterFormValues>();
  const protocol: RouterApiProtocol =
    Form.useWatch("apiProtocol", form) ?? router?.apiProtocol ?? "rest";
  const steps = useRouterTestSteps(form, protocol);
  const { save, saving } = useSaveRouter(
    form,
    router,
    steps.fingerprint,
    onClose,
  );
  const editing = router !== undefined;

  // A protocol change moves the port to that protocol's default, unless the
  // operator already typed one.
  const onValuesChange = (changed: Partial<RouterFormValues>) => {
    if (changed.apiProtocol && !form.isFieldTouched("apiPort")) {
      form.setFieldsValue({
        apiPort: PROTOCOL_DEFAULT_PORT[changed.apiProtocol],
      });
    }
    if (!editing) steps.reset();
  };

  return (
    <Modal
      open
      title={editing ? "Ubah router" : "Tambah router MikroTik"}
      onCancel={onClose}
      footer={[
        !editing && (
          <Button
            key="test"
            onClick={steps.runFirstStep}
            loading={steps.testing && !steps.first}
          >
            Uji koneksi
          </Button>
        ),
        <Button
          key="save"
          type="primary"
          disabled={!editing && steps.fingerprint === null}
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
        initialValues={initialValues(router)}
      >
        <RouterFormFields editing={editing} />
      </Form>
      <TestStepResults steps={steps} protocol={protocol} />
    </Modal>
  );
}
