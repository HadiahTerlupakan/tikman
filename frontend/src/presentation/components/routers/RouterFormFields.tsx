import { Alert, Form, Input, InputNumber, Radio, Switch } from "antd";
import type { RouterApiProtocol, RouterRole } from "@/domain/entities";
import { ROLE_LABEL } from "./routerRole";

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

// Only an existing router is offered NetFlow: the add flow is about reaching
// the router at all, and flow collection is switched on once it is in place.
function FlowFields() {
  return (
    <>
      <Form.Item
        name="flowEnabled"
        label="Kumpulkan NetFlow"
        valuePropName="checked"
      >
        <Switch />
      </Form.Item>
      <Form.Item
        name="flowSampling"
        label="Rasio sampling (1:N)"
        rules={[{ required: true, type: "number", min: 1, max: 10000 }]}
        extra="Harus sama dengan sampling di /ip traffic-flow router: N = (sampling-interval + sampling-space) / sampling-interval."
      >
        <InputNumber min={1} max={10000} />
      </Form.Item>
    </>
  );
}

// Editing never shows the stored secrets, so an empty field means "unchanged"
// there and only a new router has to be given them.
const ROLE_OPTIONS = [
  { label: "Otomatis", value: "auto" },
  ...(Object.keys(ROLE_LABEL) as RouterRole[]).map((role) => ({
    label: ROLE_LABEL[role],
    value: role,
  })),
];

// The guess is read from what the router has polled, so a new router has none to show.
function guessText(editing: boolean, guess: RouterRole | null) {
  if (!editing) return undefined;
  return guess ? `Tebakan TikMan: ${ROLE_LABEL[guess]}` : "Belum ditebak";
}

function RoleField(props: { editing: boolean; guess: RouterRole | null }) {
  return (
    <Form.Item
      name="role"
      label="Peran"
      extra={guessText(props.editing, props.guess)}
    >
      <Radio.Group options={ROLE_OPTIONS} />
    </Form.Item>
  );
}

export function RouterFormFields(props: {
  editing: boolean;
  roleGuess: RouterRole | null;
}) {
  const { editing, roleGuess } = props;
  const secretRule = editing ? [] : [{ required: true }];
  const secretExtra = editing ? KEEP_STORED : undefined;
  return (
    <>
      <Form.Item name="name" label="Nama" rules={[{ required: true, min: 2 }]}>
        <Input />
      </Form.Item>
      <RoleField editing={editing} guess={roleGuess} />
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
      {editing && (
        <>
          <Form.Item
            name="rackPosition"
            label="Posisi di rak"
            extra="Urutan dari atas di halaman MikroTik; kosongkan untuk mengurut menurut nama"
          >
            <InputNumber min={1} max={99} />
          </Form.Item>
          <FlowFields />
        </>
      )}
    </>
  );
}
