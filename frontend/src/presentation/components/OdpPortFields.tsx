import { Form, Select } from "antd";
import {
  useOdpSubscribers,
  useOdps,
} from "@/application/hooks/useDistribution";

interface OdpPortFieldsProps {
  /**
   * The ONT being placed. Its own port is not shown as taken, so reopening the
   * form on an existing pairing does not read as a conflict with itself.
   */
  currentOntId?: string;
  /** Whether a box must be chosen, as it must be when that is the whole form. */
  required?: boolean;
}

/**
 * Which box a subscriber's drop lands in, and on which port.
 *
 * The ports another subscriber already holds are listed but disabled, naming
 * who holds them: the server refuses a taken port anyway, and an operator
 * standing at the box needs to know which one to move to.
 */
export function OdpPortFields({
  currentOntId,
  required = false,
}: OdpPortFieldsProps) {
  const form = Form.useFormInstance();
  const odpId = Form.useWatch<string | undefined>("odpId");
  const { data: odps, isLoading } = useOdps();
  const { data: subscribers } = useOdpSubscribers(odpId);
  const chosen = odps?.find((odp) => odp.id === odpId);

  const taken = new Map(
    (subscribers ?? [])
      .filter((ont) => ont.id !== currentOntId && ont.odpPort)
      .map((ont) => [ont.odpPort as number, ont.serialNumber]),
  );

  return (
    <>
      <Form.Item
        name="odpId"
        label="ODP"
        rules={required ? [{ required: true, message: "Pilih ODP-nya" }] : []}
      >
        <Select
          allowClear
          showSearch
          optionFilterProp="label"
          loading={isLoading}
          placeholder={required ? "Pilih ODP" : "Belum dipasang"}
          // Clearing the box has to clear the port with it: the server refuses
          // one half of the pairing, and a stale port would be that half.
          onChange={() => form.setFieldValue("odpPort", undefined)}
          options={(odps ?? []).map((odp) => ({
            value: odp.id,
            label: `${odp.code} · ${odp.portCount - odp.usedPorts} port kosong`,
          }))}
        />
      </Form.Item>
      {chosen && (
        <Form.Item
          name="odpPort"
          label="Port"
          rules={[{ required: true, message: "Pilih portnya" }]}
        >
          <Select
            placeholder="Pilih port"
            options={Array.from(
              { length: chosen.portCount },
              (_, index) => index + 1,
            ).map((port) => ({
              value: port,
              label: taken.has(port)
                ? `Port ${port} · dipakai ${taken.get(port)}`
                : `Port ${port}`,
              disabled: taken.has(port),
            }))}
          />
        </Form.Item>
      )}
    </>
  );
}
