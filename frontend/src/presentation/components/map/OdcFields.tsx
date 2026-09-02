import { Form, Select } from "antd";
import type { FormInstance } from "antd";
import { useSites } from "@/application/hooks";
import { SPLITTER_RATIOS } from "./plantForm";
import { PonAddressFields } from "./PonAddressFields";

/**
 * What a cabinet is: where it belongs, and the PON port feeding it.
 *
 * The feed is optional. Recording where a cabinet stands before its feeder is
 * spliced is ordinary field order, and odcFeeds sends nothing until the whole
 * address is named.
 */
export function OdcFields({ form }: { form: FormInstance }) {
  const { data: sites } = useSites();
  const slot = Form.useWatch("slot", form);

  return (
    <>
      <Form.Item
        name="siteId"
        label="Site"
        rules={[{ required: true, message: "Pilih site" }]}
      >
        <Select
          placeholder="Pilih site"
          options={(sites ?? []).map((site) => ({
            value: site.id,
            label: site.name,
          }))}
        />
      </Form.Item>

      <PonAddressFields
        form={form}
        oltLabel="Disuplai OLT"
        oltExtra="Boleh dikosongkan bila feedernya belum disambung."
        allowClear
      />

      <Form.Item name="splitterOutputs" label="Rasio splitter di ODC">
        <Select
          placeholder="Pilih rasio"
          disabled={slot === undefined}
          options={SPLITTER_RATIOS.map((outputs) => ({
            value: outputs,
            label: `1:${outputs}`,
          }))}
        />
      </Form.Item>
    </>
  );
}
