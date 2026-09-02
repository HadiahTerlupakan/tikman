import { useEffect } from "react";
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
  type Coordinates,
  type OdpFormValues,
} from "./plantForm";

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

  const submit = async () => {
    const values = await form.validateFields();
    if (kind === "odc") {
      await createOdc.mutateAsync({
        siteId: values.siteId,
        name: values.name,
        code: values.code || undefined,
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

      <Form form={form} layout="vertical" initialValues={{ parentKind: "odc" }}>
        <Form.Item
          name="name"
          label="Nama"
          rules={[{ required: true, message: "Nama wajib diisi" }]}
        >
          <Input placeholder={kind === "odc" ? "ODC Cariu 1" : "ODP-CRU-012"} />
        </Form.Item>

        <Form.Item name="code" label="Kode" extra="Boleh dikosongkan.">
          <Input placeholder="ODC-CRU-01" />
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
              label="Jumlah port splitter"
              extra="1:8 berarti 8, 1:16 berarti 16."
              rules={[{ required: true, message: "Jumlah port wajib diisi" }]}
            >
              <InputNumber min={1} max={128} style={{ width: "100%" }} />
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
                    label: odc.name,
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

        <Form.Item name="address" label="Alamat">
          <Input placeholder="Alamat atau patokan" />
        </Form.Item>
        <Form.Item name="notes" label="Catatan">
          <Input.TextArea rows={2} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
