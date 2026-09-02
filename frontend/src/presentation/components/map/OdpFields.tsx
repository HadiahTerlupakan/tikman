import { Form, Radio, Select } from "antd";
import type { FormInstance } from "antd";
import { useOdcs } from "@/application/hooks";
import { SPLITTER_RATIOS } from "./plantForm";
import { PonAddressFields } from "./PonAddressFields";

/**
 * What a distribution box is: its splitter, and the one parent it hangs off.
 *
 * Both ways of hanging it stay on screen as the operator switches between them;
 * buildOdpDto sends only the one chosen, because naming two parents is refused
 * by the server and by the database alike.
 */
export function OdpFields({ form }: { form: FormInstance }) {
  const { data: odcs } = useOdcs();
  const parentKind = Form.useWatch("parentKind", form) ?? "odc";

  return (
    <>
      <Form.Item
        name="portCount"
        label="Rasio splitter"
        rules={[{ required: true, message: "Pilih rasio splitternya" }]}
      >
        <Select
          placeholder="Pilih rasio"
          options={SPLITTER_RATIOS.map((outputs) => ({
            value: outputs,
            label: `1:${outputs}`,
          }))}
        />
      </Form.Item>

      <Form.Item name="parentKind" label="Menggantung pada">
        <Radio.Group optionType="button">
          <Radio.Button value="odc">ODC</Radio.Button>
          <Radio.Button value="pon">PON port langsung</Radio.Button>
        </Radio.Group>
      </Form.Item>

      {parentKind === "odc" ? (
        <Form.Item name="odcId" label="ODC induk">
          <Select
            placeholder="Pilih ODC"
            options={(odcs ?? []).map((odc) => ({
              value: odc.id,
              label: odc.code,
            }))}
          />
        </Form.Item>
      ) : (
        <PonAddressFields form={form} oltLabel="OLT" />
      )}
    </>
  );
}
