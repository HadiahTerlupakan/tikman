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
  RouterConnectionTestResult,
} from "@/domain/entities";
import {
  useCreateMikrotikRouter,
  useUpdateMikrotikRouter,
} from "@/application/hooks/useMikrotikRouters";
import { RouterFormFields } from "./RouterFormFields";
import { requestErrorText } from "./routerTestSteps";
import {
  useRouterTestSteps,
  type RouterFormValues,
} from "./useRouterTestSteps";

const DEFAULT_SNMP_PORT = 161;
const DEFAULT_API_PORT = 8443;

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

// Step 1 only reads the certificate: it must not claim REST worked.
function FingerprintStep({
  result,
  onConfirm,
  confirming,
}: {
  result: RouterConnectionTestResult;
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
          description="Cocokkan dengan hasil `/certificate print detail where name=tikman-rest` di router. Password REST baru dikirim setelah Anda mengonfirmasi."
          action={
            <Button size="small" loading={confirming} onClick={onConfirm}>
              Fingerprint cocok, uji REST
            </Button>
          }
        />
      ) : (
        <Alert
          type="error"
          message={`REST: ${result.apiError ?? "sertifikat tidak terbaca"}`}
        />
      )}
    </Space>
  );
}

function RestStep({ result }: { result: RouterConnectionTestResult }) {
  return (
    <Alert
      type={result.apiError ? "error" : "success"}
      message={
        result.apiError
          ? `REST: ${result.apiError}`
          : `REST: ${result.boardName} · RouterOS ${result.version}`
      }
    />
  );
}

function TestStepResults({
  steps,
}: {
  steps: ReturnType<typeof useRouterTestSteps>;
}) {
  const { first, second } = steps;
  if (!first) return null;
  if (!second) {
    return (
      <FingerprintStep
        result={first}
        onConfirm={steps.runSecondStep}
        confirming={steps.testing}
      />
    );
  }
  return (
    <Space direction="vertical" style={{ width: "100%" }}>
      {snmpAlert(second)}
      <RestStep result={second} />
    </Space>
  );
}

function initialValues(router?: MikrotikRouter): Partial<RouterFormValues> {
  return {
    name: router?.name,
    host: router?.host,
    snmpPort: router?.snmpPort ?? DEFAULT_SNMP_PORT,
    apiUsername: router?.apiUsername,
    apiPort: router?.apiPort ?? DEFAULT_API_PORT,
  };
}

// A new router is saved only with the fingerprint both test steps passed on.
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
      } else if (fingerprint) {
        await create.mutateAsync({ ...values, tlsFingerprint: fingerprint });
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
  const steps = useRouterTestSteps(form);
  const { save, saving } = useSaveRouter(
    form,
    router,
    steps.fingerprint,
    onClose,
  );
  const editing = router !== undefined;

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
          disabled={!editing && !steps.fingerprint}
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
        onValuesChange={() => !editing && steps.reset()}
        initialValues={initialValues(router)}
      >
        <RouterFormFields editing={editing} />
      </Form>
      <TestStepResults steps={steps} />
    </Modal>
  );
}
