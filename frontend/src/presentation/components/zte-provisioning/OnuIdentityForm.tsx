import { Form, Input, InputNumber, Radio, Switch } from "antd";
import type { ZteProvisionTarget } from "@/domain/entities";

interface OnuIdentityFormProps {
  target: ZteProvisionTarget;
}

export function OnuIdentityForm({ target }: OnuIdentityFormProps) {
  const mode = Form.useWatch("onuIdMode");
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
      <Form.Item name="onuId" label="ONU ID">
        <InputNumber
          min={1}
          max={127}
          disabled={mode !== "custom"}
          style={{ width: "100%" }}
        />
      </Form.Item>
      <Form.Item
        name="serialNumber"
        label="Serial number"
        rules={[{ required: true }]}
      >
        <Input maxLength={12} />
      </Form.Item>
      <Form.Item name="onuType" label="ONU type" rules={[{ required: true }]}>
        <Input maxLength={64} />
      </Form.Item>
      <Form.Item name="useVeip" label="Use VEIP" valuePropName="checked">
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
