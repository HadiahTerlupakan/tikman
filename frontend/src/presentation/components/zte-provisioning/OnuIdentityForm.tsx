import { Form, Input, InputNumber, Radio, Select, Switch } from "antd";
import type { ZteProvisionTarget } from "@/domain/entities";
import { useOltOnuTypes } from "@/application/hooks/useOlts";

interface OnuIdentityFormProps {
  target: ZteProvisionTarget;
}

export function OnuIdentityForm({ target }: OnuIdentityFormProps) {
  const mode = Form.useWatch("onuIdMode");
  const { data: onuTypes } = useOltOnuTypes(target.oltId);

  // The OLT reports a model over OMCI — an F609 announces itself as F609V9 —
  // but the registration command takes one of the OLT's own onu-type names and
  // rejects anything else. The detected model is shown so the operator can
  // match it, rather than pre-selected as if it were valid.
  const detected = target.onuType?.trim();
  const typeHint = detected
    ? `The OLT reports this ONU as ${detected}. Pick the matching type it accepts.`
    : undefined;
  return (
    <>
      <Form.Item name="card" label="Card" rules={[{ required: true }]}>
        <InputNumber min={1} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item name="pon" label="PON" rules={[{ required: true }]}>
        <InputNumber min={1} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item
        name="onuIdMode"
        label="ONU ID mode"
        rules={[{ required: true }]}
      >
        <Radio.Group optionType="button" buttonStyle="solid">
          <Radio value="auto">Auto</Radio>
          <Radio value="custom">Custom</Radio>
        </Radio.Group>
      </Form.Item>
      {/* Auto sends a zero for the OLT-side allocator to replace. Showing that
          zero in a disabled box told the operator nothing and read like a
          validation error. */}
      {mode === "custom" ? (
        <Form.Item name="onuId" label="ONU ID" rules={[{ required: true }]}>
          <InputNumber min={1} max={127} style={{ width: "100%" }} />
        </Form.Item>
      ) : (
        <Form.Item
          label="ONU ID"
          extra="The lowest free ID on this PON port is assigned."
        >
          <Input disabled value="Assigned automatically" />
        </Form.Item>
      )}
      <Form.Item
        name="serialNumber"
        label="Serial number"
        rules={[{ required: true }]}
      >
        <Input maxLength={12} />
      </Form.Item>
      <Form.Item
        name="onuType"
        label="ONU type"
        extra={
          onuTypes?.length
            ? typeHint
            : "ONU types appear here once the OLT has been polled."
        }
        rules={[
          { required: true },
          {
            validator: (_, value: string) =>
              !onuTypes?.length || !value || onuTypes.includes(value)
                ? Promise.resolve()
                : Promise.reject(
                    new Error(`The OLT does not accept the type ${value}.`),
                  ),
          },
        ]}
      >
        {onuTypes?.length ? (
          <Select
            showSearch
            placeholder="Select an ONU type"
            options={onuTypes.map((name) => ({ value: name, label: name }))}
          />
        ) : (
          <Input maxLength={64} />
        )}
      </Form.Item>
      <Form.Item
        name="useVeip"
        label="Use VEIP"
        valuePropName="checked"
        extra="For ONUs that are not ZTE — Fiberhome, VSOL, Huawei and other HGUs — which present a virtual Ethernet port instead of physical ones. Binds veip_1 to the service VLAN, so it applies only to an Internet service the OLT configures over OMCI."
      >
        <Switch />
      </Form.Item>
      <Form.Item name="name" label="Name">
        <Input />
      </Form.Item>
      <Form.Item name="description" label="Description">
        <Input.TextArea rows={2} />
      </Form.Item>
      <input type="hidden" value={target.oltId} readOnly />
    </>
  );
}
