import { Form, Select } from "antd";
import type { FormInstance } from "antd";
import { useOltTopology, useOlts } from "@/application/hooks";
import { cardOptions, portOptions } from "./ponOptions";

interface PonAddressFieldsProps {
  form: FormInstance;
  /** What the OLT field is called here: a cabinet is fed, a box hangs off. */
  oltLabel: string;
  oltExtra?: string;
  allowClear?: boolean;
}

/**
 * Picks a PON port by name of the chassis that has it: OLT, then card, then
 * port, each list read from the topology the poller already stored.
 *
 * Shared because a cabinet's feed and a distribution box's parent are the same
 * address, asked the same way. Free number fields here recorded plant hanging
 * off cards a chassis does not have.
 */
export function PonAddressFields({
  form,
  oltLabel,
  oltExtra,
  allowClear,
}: PonAddressFieldsProps) {
  const { data: olts } = useOlts();
  const oltId = Form.useWatch("oltId", form);
  const slot = Form.useWatch("slot", form);
  const { data: topology, isLoading } = useOltTopology(oltId);
  const cards = cardOptions(topology);
  const ports = portOptions(topology, slot);

  return (
    <>
      <Form.Item name="oltId" label={oltLabel} extra={oltExtra}>
        <Select
          allowClear={allowClear}
          placeholder="Pilih OLT"
          // The card and port below belong to whichever OLT is chosen, so
          // neither can survive a change of chassis.
          onChange={() =>
            form.setFieldsValue({ slot: undefined, portId: undefined })
          }
          options={(olts ?? []).map((olt) => ({
            value: olt.id,
            label: olt.name,
          }))}
        />
      </Form.Item>

      <Form.Item
        name="slot"
        label="Card"
        extra={
          oltId && !isLoading && cards.length === 0
            ? "OLT ini belum pernah di-discover, jadi kartunya belum diketahui."
            : undefined
        }
      >
        <Select
          placeholder={isLoading ? "Membaca kartu..." : "Pilih card"}
          disabled={!oltId}
          loading={isLoading}
          onChange={() => form.setFieldValue("portId", undefined)}
          options={cards}
        />
      </Form.Item>

      <Form.Item name="portId" label="PON port">
        <Select
          placeholder="Pilih PON port"
          disabled={slot === undefined}
          options={ports}
        />
      </Form.Item>
    </>
  );
}
