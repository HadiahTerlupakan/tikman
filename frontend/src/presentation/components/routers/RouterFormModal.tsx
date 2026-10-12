import { useState } from "react";
import {
  Alert,
  Button,
  Form,
  Input,
  InputNumber,
  Modal,
  Space,
  message,
} from "antd";
import type { AxiosError } from "axios";
import type {
  MikrotikRouter,
  RouterConnectionTestResult,
} from "@/domain/entities";
import {
  useCreateMikrotikRouter,
  useTestRouterConnection,
  useUpdateMikrotikRouter,
} from "@/application/hooks/useMikrotikRouters";
import { confirmedFingerprint } from "./routerTestSteps";

const DEFAULT_SNMP_PORT = 161;
const DEFAULT_API_PORT = 8443;

interface FormValues {
  name: string;
  host: string;
  snmpCommunity: string;
  snmpPort: number;
  apiUsername: string;
  apiPassword: string;
  apiPort: number;
}

interface Props {
  router?: MikrotikRouter;
  onClose: () => void;
}

function errorText(error: unknown): string {
  return (
    (error as AxiosError<{ error?: string }>).response?.data?.error ??
    "Permintaan gagal"
  );
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

export function RouterFormModal({ router, onClose }: Props) {
  const [form] = Form.useForm<FormValues>();
  const create = useCreateMikrotikRouter();
  const update = useUpdateMikrotikRouter();
  const test = useTestRouterConnection();
  const [first, setFirst] = useState<RouterConnectionTestResult | null>(null);
  const [second, setSecond] = useState<RouterConnectionTestResult | null>(null);
  const editing = router !== undefined;
  const fingerprint = confirmedFingerprint(first, second);

  const runFirstStep = async () => {
    try {
      setSecond(null);
      setFirst(await test.mutateAsync(await form.validateFields()));
    } catch (error) {
      message.error(errorText(error));
    }
  };

  const runSecondStep = async () => {
    try {
      const values = await form.validateFields();
      setSecond(
        await test.mutateAsync({
          ...values,
          tlsFingerprint: first?.fingerprint,
        }),
      );
    } catch (error) {
      message.error(errorText(error));
    }
  };

  const save = async () => {
    try {
      const values = await form.validateFields();
      if (editing) {
        await update.mutateAsync({ id: router.id, data: values });
      } else if (fingerprint) {
        await create.mutateAsync({ ...values, tlsFingerprint: fingerprint });
      }
      onClose();
    } catch (error) {
      message.error(errorText(error));
    }
  };

  const resetSteps = () => {
    setFirst(null);
    setSecond(null);
  };

  const secretRule = editing ? [] : [{ required: true }];
  return (
    <Modal
      open
      title={editing ? "Ubah router" : "Tambah router MikroTik"}
      onCancel={onClose}
      footer={[
        !editing && (
          <Button
            key="test"
            onClick={runFirstStep}
            loading={test.isPending && !first}
          >
            Uji koneksi
          </Button>
        ),
        <Button
          key="save"
          type="primary"
          disabled={!editing && !fingerprint}
          loading={create.isPending || update.isPending}
          onClick={save}
        >
          Simpan
        </Button>,
      ]}
    >
      <Form
        form={form}
        layout="vertical"
        onValuesChange={() => !editing && resetSteps()}
        initialValues={{
          name: router?.name,
          host: router?.host,
          snmpPort: router?.snmpPort ?? DEFAULT_SNMP_PORT,
          apiUsername: router?.apiUsername,
          apiPort: router?.apiPort ?? DEFAULT_API_PORT,
        }}
      >
        <Form.Item
          name="name"
          label="Nama"
          rules={[{ required: true, min: 2 }]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          name="host"
          label="IP atau hostname"
          rules={[{ required: true }]}
        >
          <Input />
        </Form.Item>
        <Form.Item
          name="snmpCommunity"
          label="SNMP community"
          rules={secretRule}
          extra={
            editing ? "Kosongkan untuk tetap memakai yang tersimpan" : undefined
          }
        >
          <Input.Password autoComplete="off" />
        </Form.Item>
        <Form.Item name="snmpPort" label="Port SNMP">
          <InputNumber min={1} max={65535} />
        </Form.Item>
        <Form.Item
          name="apiUsername"
          label="User REST (read-only)"
          rules={[{ required: true }]}
        >
          <Input autoComplete="off" />
        </Form.Item>
        <Form.Item
          name="apiPassword"
          label="Password REST"
          rules={secretRule}
          extra={
            editing ? "Kosongkan untuk tetap memakai yang tersimpan" : undefined
          }
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>
        <Form.Item name="apiPort" label="Port REST (www-ssl)">
          <InputNumber min={1} max={65535} />
        </Form.Item>
      </Form>
      {first && !second && (
        <FingerprintStep
          result={first}
          onConfirm={runSecondStep}
          confirming={test.isPending}
        />
      )}
      {first && second && (
        <Space direction="vertical" style={{ width: "100%" }}>
          {snmpAlert(second)}
          <RestStep result={second} />
        </Space>
      )}
    </Modal>
  );
}
