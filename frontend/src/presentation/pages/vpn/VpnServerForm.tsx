import { Alert, Button, Form, Input, InputNumber, Space, message } from "antd";
import { useSaveWireguardServer } from "@/application/hooks";
import type {
  SaveWireguardServerDto,
  WireguardServer,
} from "@/domain/entities";

const DEFAULT_LISTEN_PORT = 51820;

const PORT_HINT =
  "Port UDP di bawah harus sama dengan WIREGUARD_PORT di berkas .env deployment dan dibuka di firewall penyedia VPS.";

interface Props {
  /** The server being edited, or undefined for the one-time setup. */
  server?: WireguardServer;
  onDone?: () => void;
}

export function VpnServerForm({ server, onDone }: Props) {
  const saveServer = useSaveWireguardServer();
  const [form] = Form.useForm<SaveWireguardServerDto>();

  return (
    <>
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={server ? "Ubah pengaturan server" : "Isi sekali saja"}
        description={
          server ? PORT_HINT : `Kunci server dibuat otomatis. ${PORT_HINT}`
        }
      />
      <Form
        form={form}
        layout="vertical"
        initialValues={{
          endpointHost: server?.endpointHost ?? window.location.hostname,
          listenPort: server?.listenPort ?? DEFAULT_LISTEN_PORT,
        }}
        onFinish={(values) =>
          saveServer.mutate(values, {
            onSuccess: () => onDone?.(),
            onError: () => message.error("Gagal menyimpan pengaturan VPN"),
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
        <Space>
          <Button
            type="primary"
            htmlType="submit"
            loading={saveServer.isPending}
          >
            {server ? "Simpan" : "Aktifkan"}
          </Button>
          {server && <Button onClick={onDone}>Batal</Button>}
        </Space>
      </Form>
    </>
  );
}
