import { useState } from "react";
import { Alert, Button, Card, Descriptions } from "antd";
import { useWireguardServer } from "@/application/hooks";
import { ApiError } from "@/infrastructure/http";
import { VpnServerForm } from "./VpnServerForm";

// GET /wireguard/server answers 404 with code NOT_CONFIGURED only before the
// one-time setup has run; the shared error mapper collapses every 404 into
// NOT_FOUND, so the status is what survives to key on. A transient 500 must not
// be read as "no server yet": the setup form would offer to overwrite a working
// endpoint and port.
function isNotConfigured(error: unknown): boolean {
  return error instanceof ApiError && error.statusCode === 404;
}

export function VpnServerCard() {
  const { data: server, isLoading, error } = useWireguardServer();
  const [editing, setEditing] = useState(false);

  if (isLoading) {
    return <Card loading title="Server VPN" />;
  }

  if (isNotConfigured(error)) {
    return (
      <Card title="Aktifkan VPN">
        <VpnServerForm />
      </Card>
    );
  }

  if (!server) {
    return (
      <Card title="Server VPN">
        <Alert
          type="error"
          showIcon
          message="Gagal memuat pengaturan server VPN"
          description="Muat ulang halaman. Jangan isi ulang formulir aktivasi selama pengaturan belum tampil: konfigurasi yang sudah berjalan bisa tertimpa."
        />
      </Card>
    );
  }

  if (editing) {
    return (
      <Card title="Server VPN">
        <VpnServerForm server={server} onDone={() => setEditing(false)} />
      </Card>
    );
  }

  return (
    <Card
      title="Server VPN"
      extra={
        <Button size="small" onClick={() => setEditing(true)}>
          Ubah
        </Button>
      }
    >
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
