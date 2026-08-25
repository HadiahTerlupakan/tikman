import { Form, Input, InputNumber, Radio } from "antd";

export function InternetServiceForm() {
  return (
    <>
      <strong>Service 1 — Internet</strong>
      <Form.Item
        name="vlanMode"
        label="VLAN mode"
        initialValue="tag"
        rules={[{ required: true }]}
      >
        <Radio.Group disabled options={[{ value: "tag", label: "Tag" }]} />
      </Form.Item>
      <Form.Item
        name="serviceType"
        label="Service type"
        initialValue="internet"
      >
        <Radio.Group
          disabled
          options={[{ value: "internet", label: "Internet" }]}
        />
      </Form.Item>
      <Form.Item name="vlanId" label="VLAN ID" rules={[{ required: true }]}>
        <InputNumber min={1} max={4094} style={{ width: "100%" }} />
      </Form.Item>
      <Form.Item
        name="downloadProfile"
        label="Download profile"
        rules={[{ required: true }]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name="uploadProfile"
        label="Upload profile"
        rules={[{ required: true }]}
      >
        <Input />
      </Form.Item>
      <Form.Item name="wanMode" label="WAN mode" initialValue="pppoe">
        <Radio.Group disabled options={[{ value: "pppoe", label: "PPPoE" }]} />
      </Form.Item>
      <Form.Item
        name="vlanProfile"
        label="VLAN profile"
        rules={[{ required: true }]}
      >
        <Input />
      </Form.Item>
      <Form.Item
        name="pppoeUsername"
        label="PPPoE username"
        rules={[{ required: true }]}
      >
        <Input autoComplete="off" />
      </Form.Item>
      <Form.Item
        name="pppoePassword"
        label="PPPoE password"
        rules={[{ required: true }]}
      >
        <Input.Password autoComplete="new-password" />
      </Form.Item>
    </>
  );
}
