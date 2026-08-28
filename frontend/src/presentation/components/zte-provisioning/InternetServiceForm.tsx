import { Form, Input, InputNumber, Radio, Select } from "antd";
import {
  useOltTcontProfiles,
  useOltVlanProfiles,
  useOltVlans,
} from "@/application/hooks/useOlts";

interface InternetServiceFormProps {
  oltId?: string;
}

// Untagged traffic and a bridged ONU both leave the WAN to the ONT: the OLT
// only carries their traffic, so the OMCI service and WAN fields do not apply.
function carriesOmciWan(vlanMode?: string, serviceType?: string) {
  return vlanMode === "tag" && serviceType === "internet";
}

export function InternetServiceForm({ oltId }: InternetServiceFormProps) {
  const form = Form.useFormInstance();
  const vlanMode = Form.useWatch("vlanMode", form);
  const serviceType = Form.useWatch("serviceType", form);
  const wanMode = Form.useWatch("wanMode", form);
  const wanIpMode = Form.useWatch("wanIpMode", form);

  const { data: vlans } = useOltVlans(oltId);
  const { data: profiles } = useOltTcontProfiles(oltId);
  const { data: vlanProfiles } = useOltVlanProfiles(oltId);

  const toOptions = (names?: string[]) =>
    names?.map((name) => ({ value: name, label: name }));
  const profileOptions = toOptions(profiles);
  const vlanProfileOptions = toOptions(vlanProfiles);

  const omciWan = carriesOmciWan(vlanMode, serviceType);
  const configuredByOlt = omciWan && wanMode === "wan_ip";

  return (
    <>
      <strong>Service 1</strong>
      <Form.Item
        name="vlanMode"
        label="VLAN mode"
        initialValue="tag"
        rules={[{ required: true }]}
      >
        <Radio.Group
          optionType="button"
          options={[
            { value: "tag", label: "Tag" },
            { value: "untag", label: "Untag" },
          ]}
          onChange={() =>
            form.setFieldsValue({ wanMode: "setup_via_ont", wanIpMode: "" })
          }
        />
      </Form.Item>

      {vlanMode === "tag" && (
        <Form.Item
          name="serviceType"
          label="Service type"
          initialValue="internet"
        >
          <Radio.Group
            optionType="button"
            options={[
              { value: "internet", label: "Internet" },
              { value: "bridge", label: "Bridge" },
            ]}
            onChange={(event) =>
              form.setFieldsValue({
                wanMode:
                  event.target.value === "internet"
                    ? "wan_ip"
                    : "setup_via_ont",
                wanIpMode: event.target.value === "internet" ? "pppoe" : "",
              })
            }
          />
        </Form.Item>
      )}

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

      <Form.Item name="wanMode" label="WAN mode">
        <Radio.Group
          optionType="button"
          options={[
            { value: "wan_ip", label: "WAN-IP", disabled: !omciWan },
            { value: "setup_via_ont", label: "Setup via ONT" },
          ]}
        />
      </Form.Item>

      {configuredByOlt && (
        <>
          <Form.Item name="wanIpMode" label="Mode WAN-IP" initialValue="pppoe">
            <Radio.Group
              optionType="button"
              options={[
                { value: "pppoe", label: "PPPoE" },
                { value: "dhcp", label: "DHCP" },
                { value: "static", label: "Static" },
              ]}
            />
          </Form.Item>

          <Form.Item
            name="vlanProfile"
            label="VLAN profile"
            rules={[{ required: true }]}
            extra={
              vlanProfileOptions?.length
                ? undefined
                : "VLAN profiles appear here once the OLT has been polled."
            }
          >
            {vlanProfileOptions?.length ? (
              <Select
                showSearch
                placeholder="Select a VLAN profile"
                options={vlanProfileOptions}
              />
            ) : (
              <Input />
            )}
          </Form.Item>
        </>
      )}

      {configuredByOlt && wanIpMode === "pppoe" && (
        <>
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
      )}
    </>
  );
}
