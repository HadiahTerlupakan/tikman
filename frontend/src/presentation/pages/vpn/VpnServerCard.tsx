import {
  Alert,
  Button,
  Card,
  Descriptions,
  Form,
  Input,
  InputNumber,
  message,
} from "antd";
import {
  useSaveWireguardServer,
  useWireguardServer,
} from "@/application/hooks";
import type { SaveWireguardServerDto } from "@/domain/entities";

const DEFAULT_LISTEN_PORT = 51820;

export function VpnServerCard() {
  const { data: server, isLoading } = useWireguardServer();
  const saveServer = useSaveWireguardServer();
  const [form] = Form.useForm<SaveWireguardServerDto>();

  if (isLoading) {
    return <Card loading title="Server VPN" />;
  }

  if (!server) {
    return (
      <Card title="Aktifkan VPN">
        <Alert
          type="info"
          showIcon
          style={{ marginBottom: 16 }}
          message="Isi sekali saja"
          description="Kunci server dibuat otomatis. Port UDP di bawah harus sama dengan WIREGUARD_PORT di berkas .env deployment dan dibuka di firewall penyedia VPS."
        />
        <Form
          form={form}
          layout="vertical"
          initialValues={{
            endpointHost: window.location.hostname,
            listenPort: DEFAULT_LISTEN_PORT,
          }}
          onFinish={(values) =>
            saveServer.mutate(values, {
              onError: () => message.error("Gagal mengaktifkan VPN"),
            })
          }
        >
          <Form.Item
            name="endpointHost"
            label="Alamat publik VPS"
            rules={[{ required: true, message: "Alamat publik wajib diisi" }]}
          >
            <Input placeholder="vpn.contoh.id" />
          </Form.Item>
          <Form.Item
            name="listenPort"
            label="Port UDP"
            rules={[{ required: true, message: "Port wajib diisi" }]}
          >
            <InputNumber min={1} max={65535} style={{ width: "100%" }} />
          </Form.Item>
          <Button
            type="primary"
            htmlType="submit"
            loading={saveServer.isPending}
          >
            Aktifkan
          </Button>
        </Form>
      </Card>
    );
  }

  return (
    <Card title="Server VPN">
      <Descriptions column={2} size="small">
        <Descriptions.Item label="Alamat publik">
          {server.endpointHost}:{server.listenPort}
        </Descriptions.Item>
        <Descriptions.Item label="Subnet tunnel">
          {server.tunnelSubnet}
        </Descriptions.Item>
        <Descriptions.Item label="Public key" span={2}>
          <code>{server.publicKey}</code>
        </Descriptions.Item>
      </Descriptions>
    </Card>
  );
}
