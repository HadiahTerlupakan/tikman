import { useEffect, useState } from "react";
import { Alert, Collapse, Form, Input, Modal, Select } from "antd";
import {
  useCreateWireguardPeer,
  useSites,
  useSuggestedSubnets,
  useUpdateWireguardPeer,
} from "@/application/hooks";
import type { WireguardPeer } from "@/domain/entities";

interface Props {
  open: boolean;
  /** The peer being edited, or null/undefined to register a new one. */
  peer?: WireguardPeer | null;
  onClose: () => void;
}

interface FormValues {
  siteId: string;
  name: string;
  allowedIps: string;
  tunnelAddress?: string;
}

export function VpnPeerFormModal({ open, peer, onClose }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [suggestFor, setSuggestFor] = useState<string | undefined>();
  const { data: sites } = useSites();
  const { data: suggested } = useSuggestedSubnets(suggestFor);
  const createPeer = useCreateWireguardPeer();
  const updatePeer = useUpdateWireguardPeer();

  const isEdit = !!peer;
  const mutation = isEdit ? updatePeer : createPeer;

  const handleSiteChange = (value: string) => {
    setSuggestFor(value);
    // Clear immediately: a suggestion derived from the previous site must never
    // sit in the field under a different site's name.
    form.setFieldValue("allowedIps", "");
    // The site's name is the right default and the wrong answer for the second
    // POP, so it is offered rather than imposed.
    form.setFieldValue(
      "name",
      sites?.find((site) => site.id === value)?.name ?? "",
    );
  };

  // Editing shows the peer's stored subnets and asks for no suggestion: the
  // operator opened this form precisely because the suggested value was wrong.
  useEffect(() => {
    if (!open) {
      return;
    }
    setSuggestFor(undefined);
    form.setFieldsValue({
      siteId: peer?.siteId,
      name: peer?.name ?? "",
      allowedIps: peer?.allowedIps.join(", ") ?? "",
      tunnelAddress: undefined,
    });
  }, [open, peer, form]);

  // The suggestion comes from the OLT addresses already registered for the site,
  // so the operator confirms a value instead of inventing one.
  useEffect(() => {
    if (!suggested) {
      return;
    }
    form.setFieldValue("allowedIps", suggested.join(", "));
  }, [suggested, form]);

  const closeAndReset = () => {
    form.resetFields();
    setSuggestFor(undefined);
    onClose();
  };

  const submit = async () => {
    const values = await form.validateFields();
    const name = values.name.trim();
    const allowedIps = values.allowedIps
      .split(",")
      .map((entry) => entry.trim())
      .filter((entry) => entry !== "");

    if (peer) {
      await updatePeer.mutateAsync({ id: peer.id, data: { name, allowedIps } });
    } else {
      await createPeer.mutateAsync({
        siteId: values.siteId,
        name,
        allowedIps,
        tunnelAddress: values.tunnelAddress || undefined,
      });
    }
    closeAndReset();
  };

  return (
    <Modal
      open={open}
      title={isEdit ? "Sunting tunnel" : "Tambah tunnel"}
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={mutation.isPending}
      onOk={submit}
      onCancel={closeAndReset}
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
      <Form form={form} layout="vertical">
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: "Pilih site" }]}
          extra={
            isEdit
              ? "Site tidak bisa dipindah. Hapus tunnel lalu buat baru."
              : undefined
          }
        >
          <Select
            placeholder="Pilih site"
            disabled={isEdit}
            onChange={handleSiteChange}
            options={sites?.map((site) => ({
              value: site.id,
              label: site.name,
            }))}
          />
        </Form.Item>
        <Form.Item
          name="name"
          label="Nama tunnel"
          extra="Beri nama berbeda bila satu site punya lebih dari satu POP."
          rules={[{ required: true, message: "Nama tunnel wajib diisi" }]}
        >
          <Input placeholder="Cariu POP 1" />
        </Form.Item>
        <Form.Item
          name="allowedIps"
          label="Subnet lokal di site"
          extra="Dipisah koma bila lebih dari satu. Nilai ini disarankan dari alamat OLT yang sudah terdaftar."
          rules={[{ required: true, message: "Subnet wajib diisi" }]}
        >
          <Input placeholder="10.10.10.0/24" />
        </Form.Item>
        {!isEdit && (
          <Collapse
            ghost
            items={[
              {
                key: "advanced",
                label: "Lanjutan",
                children: (
                  <Form.Item
                    name="tunnelAddress"
                    label="Alamat tunnel"
                    extra="Kosongkan agar dipilih otomatis."
                  >
                    <Input placeholder="10.88.0.2" />
                  </Form.Item>
                ),
              },
            ]}
          />
        )}
      </Form>
    </Modal>
  );
}
