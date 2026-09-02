import { useCallback, useEffect } from "react";
import { Alert, Form, Input, InputNumber, Modal, Radio, Select } from "antd";
import {
  useCreateOdc,
  useCreateOdp,
  useOdcs,
  useOlts,
  useSites,
} from "@/application/hooks";
import {
  buildOdpDto,
  odpFormProblem,
  SPLITTER_RATIOS,
  type Coordinates,
  type OdpFormValues,
} from "./plantForm";
import { AddressResolver } from "./AddressResolver";

export type PlantKind = "odc" | "odp";

interface PlantFormModalProps {
  open: boolean;
  kind: PlantKind;
  /** Where the operator clicked on the map. */
  coordinates?: Coordinates;
  onClose: () => void;
}

/**
 * Records a cabinet or a distribution box at the point the map was clicked.
 *
 * Placing by click rather than by typing coordinates: this network's plant is
 * being recorded from nothing, and asking someone to key latitude and longitude
 * for several hundred boxes is asking for both tedium and typos.
 */
export function PlantFormModal({
  open,
  kind,
  coordinates,
  onClose,
}: PlantFormModalProps) {
  const [form] = Form.useForm();
  const { data: sites } = useSites();
  const { data: odcs } = useOdcs();
  const { data: olts } = useOlts();
  const createOdc = useCreateOdc();
  const createOdp = useCreateOdp();

  const mutation = kind === "odc" ? createOdc : createOdp;
  const parentKind = Form.useWatch("parentKind", form) ?? "odc";

  useEffect(() => {
    if (open) {
      form.resetFields();
    }
  }, [open, form]);

  // Only fills a field the operator has not typed into: a resolved address
  // arriving late must never overwrite what someone corrected by hand.
  const fillAddress = useCallback(
    (address: string) => {
      if (address && !form.getFieldValue("address")) {
        form.setFieldValue("address", address);
      }
    },
    [form],
  );

  const submit = async () => {
    const values = await form.validateFields();
    if (kind === "odc") {
      await createOdc.mutateAsync({
        siteId: values.siteId,
        code: values.code,
        address: values.address || undefined,
        notes: values.notes || undefined,
        latitude: coordinates?.latitude,
        longitude: coordinates?.longitude,
      });
      onClose();
      return;
    }

    const problem = odpFormProblem(values as OdpFormValues, coordinates);
    if (problem) {
      throw new Error(problem);
    }
    await createOdp.mutateAsync(
      buildOdpDto(values as OdpFormValues, coordinates),
    );
    onClose();
  };

  return (
    <Modal
      open={open}
      title={kind === "odc" ? "Tambah ODC" : "Tambah ODP"}
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={mutation.isPending}
      onOk={submit}
      onCancel={onClose}
    >
      {mutation.isError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="Gagal menyimpan"
          description={(mutation.error as Error).message}
        />
      )}

      <Alert
        type="info"
        showIcon
        style={{ marginBottom: 16 }}
        message={
          coordinates
            ? `Lokasi: ${coordinates.latitude.toFixed(5)}, ${coordinates.longitude.toFixed(5)}`
            : "Klik di peta untuk menentukan lokasinya"
        }
      />

      {open && (
        <AddressResolver coordinates={coordinates} onResolved={fillAddress} />
      )}

      <Form form={form} layout="vertical" initialValues={{ parentKind: "odc" }}>
        <Form.Item
          name="code"
          label={kind === "odc" ? "Kode ODC" : "Kode ODP"}
          rules={[{ required: true, message: "Kode wajib diisi" }]}
        >
          <Input placeholder={kind === "odc" ? "ODC-CRU-01" : "ODP-CRU-012"} />
        </Form.Item>

        {kind === "odc" ? (
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
        ) : (
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
              <>
                <Form.Item name="oltId" label="OLT">
                  <Select
                    placeholder="Pilih OLT"
                    options={(olts ?? []).map((olt) => ({
                      value: olt.id,
                      label: olt.name,
                    }))}
                  />
                </Form.Item>
                <Form.Item name="slot" label="Slot">
                  <InputNumber min={0} style={{ width: "100%" }} />
                </Form.Item>
                <Form.Item name="portId" label="PON port">
                  <InputNumber min={0} style={{ width: "100%" }} />
                </Form.Item>
              </>
            )}
          </>
        )}

        <Form.Item
          name="address"
          label="Alamat"
          extra="Terisi otomatis dari titik yang diklik; boleh diperbaiki."
        >
          <Input placeholder="Alamat atau patokan" />
        </Form.Item>
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
