import { Form, Input, InputNumber, Radio, Select } from "antd";
import { useOltTcontProfiles, useOltVlans } from "@/application/hooks/useOlts";

interface InternetServiceFormProps {
  oltId?: string;
}

export function InternetServiceForm({ oltId }: InternetServiceFormProps) {
  const { data: vlans } = useOltVlans(oltId);
  const { data: profiles } = useOltTcontProfiles(oltId);
  const profileOptions = profiles?.map((name) => ({
    value: name,
    label: name,
  }));

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
      <Form.Item
        name="vlanId"
        label="VLAN ID"
        rules={[{ required: true }]}
        extra={
          vlans?.length
            ? undefined
            : "VLANs appear here once the OLT has been polled."
        }
      >
        {/* Falls back to a typed ID: an OLT that has never been polled, or one
            that was unreachable on its last poll, has no cached VLAN list. */}
        {vlans?.length ? (
          <Select
            showSearch
            optionFilterProp="label"
            placeholder="Select a VLAN"
            options={vlans.map((vlan) => ({
              value: vlan.vlanId,
              label: `${vlan.vlanId} — ${vlan.name}`,
            }))}
          />
        ) : (
          <InputNumber min={1} max={4094} style={{ width: "100%" }} />
        )}
      </Form.Item>
      {/* Both fields offer the OLT's T-CONT profiles: the command references one
          name, and the validator requires the two to match. */}
      <Form.Item
        name="downloadProfile"
        label="Download profile"
        rules={[{ required: true }]}
        extra={
          profileOptions?.length
            ? undefined
            : "Profiles appear here once the OLT has been polled."
        }
      >
        {profileOptions?.length ? (
          <Select
            showSearch
            placeholder="Select a T-CONT profile"
            options={profileOptions}
          />
        ) : (
          <Input />
        )}
      </Form.Item>
      <Form.Item
        name="uploadProfile"
        label="Upload profile"
        rules={[{ required: true }]}
      >
        {profileOptions?.length ? (
          <Select
            showSearch
            placeholder="Select a T-CONT profile"
            options={profileOptions}
          />
        ) : (
          <Input />
        )}
      </Form.Item>
      <Form.Item name="wanMode" label="WAN mode" initialValue="pppoe">
        <Radio.Group disabled options={[{ value: "pppoe", label: "PPPoE" }]} />
      </Form.Item>
      {/* Stays typed: a C300 V2.1.0 has no listing command for it — "show gpon
          profile ?" offers only tcont and traffic. */}
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
