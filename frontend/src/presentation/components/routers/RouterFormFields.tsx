import { Alert, Form, Input, InputNumber, Radio } from "antd";
import type { RouterApiProtocol } from "@/domain/entities";

const KEEP_STORED = "Kosongkan untuk tetap memakai yang tersimpan";

const PROTOCOL_OPTIONS = [
  { label: "REST (www-ssl) — disarankan", value: "rest" },
  { label: "API-SSL", value: "api-ssl" },
  { label: "API (tanpa enkripsi)", value: "api" },
];

// The notice follows the chosen protocol, so it reads the form it sits in.
function ProtocolNotice() {
  const protocol = Form.useWatch<RouterApiProtocol | undefined>("apiProtocol");
  if (protocol === "api") {
    return (
      <Alert
        type="warning"
        showIcon
        style={{ marginBottom: 16 }}
        message="Password dikirim tanpa enkripsi"
        description="Username dan password user read-only melewati internet apa adanya. Batasi service api di router hanya untuk IP VPS: /ip service set api address=IP_VPS/32"
      />
    );
  }
  if (protocol === "api-ssl") {
    return (
      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message="API-SSL butuh sertifikat di router"
        description="Sertifikat service api-ssl tidak boleh none: tanpa sertifikat RouterOS hanya menawarkan cipher anonim yang ditolak TikMan."
      />
    );
  }
  return null;
}

// Editing never shows the stored secrets, so an empty field means "unchanged"
// there and only a new router has to be given them.
export function RouterFormFields({ editing }: { editing: boolean }) {
  const secretRule = editing ? [] : [{ required: true }];
  const secretExtra = editing ? KEEP_STORED : undefined;
  return (
    <>
      <Form.Item name="name" label="Nama" rules={[{ required: true, min: 2 }]}>
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
        extra={secretExtra}
      >
        <Input.Password autoComplete="off" />
      </Form.Item>
      <Form.Item name="snmpPort" label="Port SNMP">
        <InputNumber min={1} max={65535} />
      </Form.Item>
      <Form.Item name="apiProtocol" label="Protokol">
        <Radio.Group optionType="button" options={PROTOCOL_OPTIONS} />
      </Form.Item>
      <ProtocolNotice />
      <Form.Item
        name="apiUsername"
        label="User API (read-only)"
        rules={[{ required: true }]}
      >
        <Input autoComplete="off" />
      </Form.Item>
      <Form.Item
        name="apiPassword"
        label="Password API"
        rules={secretRule}
        extra={secretExtra}
      >
        <Input.Password autoComplete="new-password" />
      </Form.Item>
      <Form.Item name="apiPort" label="Port API">
        <InputNumber min={1} max={65535} />
      </Form.Item>
    </>
  );
}
