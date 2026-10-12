import { Form, Input, InputNumber } from "antd";

const KEEP_STORED = "Kosongkan untuk tetap memakai yang tersimpan";

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
        extra={secretExtra}
      >
        <Input.Password autoComplete="new-password" />
      </Form.Item>
      <Form.Item name="apiPort" label="Port REST (www-ssl)">
        <InputNumber min={1} max={65535} />
      </Form.Item>
    </>
  );
}
