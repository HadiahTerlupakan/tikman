import { Alert, Form, Input, InputNumber, Radio } from "antd";
import type { SwitchProtocol, SwitchVendor } from "@/domain/entities";
import {
  PROTOCOL_LABEL,
  TELNET_WARNING,
  VENDOR_OPTIONS,
  protocolsFor,
} from "./switchProtocol";

const KEEP_STORED = "Kosongkan untuk tetap memakai yang tersimpan";

// The options and the warning follow the form's own vendor and protocol.
function ProtocolField() {
  const vendor = Form.useWatch<SwitchVendor | undefined>("vendor");
  const protocol = Form.useWatch<SwitchProtocol | undefined>("protocol");
  const options = protocolsFor(vendor ?? "huawei_ce").map((value) => ({
    label: PROTOCOL_LABEL[value],
    value,
  }));
  return (
    <>
      <Form.Item name="protocol" label="Protokol">
        <Radio.Group optionType="button" options={options} />
      </Form.Item>
      {protocol === "telnet" && (
        <Alert
          type="warning"
          showIcon
          style={{ marginBottom: 16 }}
          message={TELNET_WARNING}
        />
      )}
    </>
  );
}

// Editing never shows the stored secrets, so an empty field means "unchanged"
// there; the vendor is fixed once the switch exists.
export function SwitchFormFields({ editing }: { editing: boolean }) {
  const secretRule = editing ? [] : [{ required: true }];
  const secretExtra = editing ? KEEP_STORED : undefined;
  return (
    <>
      <Form.Item name="vendor" label="Vendor">
        <Radio.Group options={VENDOR_OPTIONS} disabled={editing} />
      </Form.Item>
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
      <ProtocolField />
      <Form.Item
        name="username"
        label="User (read-only)"
        rules={[{ required: true }]}
      >
        <Input autoComplete="off" />
      </Form.Item>
      <Form.Item
        name="password"
        label="Password"
        rules={secretRule}
        extra={secretExtra}
      >
        <Input.Password autoComplete="new-password" />
      </Form.Item>
      <Form.Item name="port" label="Port">
        <InputNumber min={1} max={65535} />
      </Form.Item>
      {editing && (
        <Form.Item
          name="rackPosition"
          label="Posisi di rak"
          extra="Urutan dari atas di halaman Switch; kosongkan untuk mengurut menurut nama"
        >
          <InputNumber min={1} max={99} />
        </Form.Item>
      )}
    </>
  );
}
