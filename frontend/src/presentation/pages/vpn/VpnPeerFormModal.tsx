import { useEffect, useState } from "react";
import { Alert, Collapse, Form, Input, Modal, Select } from "antd";
import {
  useCreateWireguardPeer,
  useSites,
  useSuggestedSubnets,
} from "@/application/hooks";

interface Props {
  open: boolean;
  onClose: () => void;
}

interface FormValues {
  siteId: string;
  allowedIps: string;
  tunnelAddress?: string;
}

export function VpnPeerFormModal({ open, onClose }: Props) {
  const [form] = Form.useForm<FormValues>();
  const [siteId, setSiteId] = useState<string | undefined>();
  const { data: sites } = useSites();
  const { data: suggested } = useSuggestedSubnets(siteId);
  const createPeer = useCreateWireguardPeer();

  const handleSiteChange = (value: string) => {
    setSiteId(value);
    // Clear immediately: a suggestion derived from the previous site must never
    // sit in the field under a different site's name.
    form.setFieldValue("allowedIps", "");
  };

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
    setSiteId(undefined);
    onClose();
  };

  const submit = async () => {
    const values = await form.validateFields();
    const site = sites?.find((candidate) => candidate.id === values.siteId);
    await createPeer.mutateAsync({
      siteId: values.siteId,
      name: site?.name ?? "Site",
      allowedIps: values.allowedIps
        .split(",")
        .map((entry) => entry.trim())
        .filter((entry) => entry !== ""),
      tunnelAddress: values.tunnelAddress || undefined,
    });
    closeAndReset();
  };

  return (
    <Modal
      open={open}
      title="Tambah site ke VPN"
      okText="Simpan"
      cancelText="Batal"
      confirmLoading={createPeer.isPending}
      onOk={submit}
      onCancel={closeAndReset}
    >
      {createPeer.isError && (
        <Alert
          type="error"
          showIcon
          style={{ marginBottom: 16 }}
          message="Gagal menyimpan"
          description={(createPeer.error as Error).message}
        />
      )}
      <Form form={form} layout="vertical">
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: "Pilih site" }]}
        >
          <Select
            placeholder="Pilih site"
            onChange={handleSiteChange}
            options={sites?.map((site) => ({
              value: site.id,
              label: site.name,
            }))}
          />
        </Form.Item>
        <Form.Item
          name="allowedIps"
          label="Subnet lokal di site"
          extra="Dipisah koma bila lebih dari satu. Nilai ini disarankan dari alamat OLT yang sudah terdaftar."
          rules={[{ required: true, message: "Subnet wajib diisi" }]}
        >
          <Input placeholder="10.10.10.0/24" />
        </Form.Item>
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
      </Form>
    </Modal>
  );
}
