import { Form, Input, Modal, Select } from "antd";
import type { WaAccount } from "@/domain/entities";
import { GROUP_NAME_MAX, parseParticipants } from "./groupSummary";

interface CreateGroupModalProps {
  open: boolean;
  accounts: WaAccount[];
  creating: boolean;
  onCreate: (vars: {
    waAccountId: string;
    name: string;
    participants: string[];
  }) => void;
  onClose: () => void;
}

interface CreateGroupValues {
  waAccountId: string;
  name: string;
  participants: string;
}

/** A new group on one of our numbers, with its first members. */
export function CreateGroupModal({
  open,
  accounts,
  creating,
  onCreate,
  onClose,
}: CreateGroupModalProps) {
  const [form] = Form.useForm<CreateGroupValues>();
  // Only a connected number can create anything; preselecting one saves the
  // common case of a single number a click.
  const connected = accounts.filter((a) => a.status === "connected");

  return (
    <Modal
      open={open}
      title="Buat grup"
      okText="Buat"
      cancelText="Batal"
      confirmLoading={creating}
      onOk={() => form.submit()}
      onCancel={onClose}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        preserve={false}
        initialValues={{ waAccountId: connected[0]?.id }}
        onFinish={(values) =>
          onCreate({
            waAccountId: values.waAccountId,
            name: values.name.trim(),
            participants: parseParticipants(values.participants),
          })
        }
      >
        <Form.Item
          name="waAccountId"
          label="Nomor WhatsApp"
          rules={[{ required: true, message: "Pilih nomor yang membuat grup" }]}
        >
          <Select
            options={connected.map((a) => ({ value: a.id, label: a.label }))}
            placeholder="Pilih nomor"
            notFoundContent="Tidak ada nomor yang tersambung"
          />
        </Form.Item>
        <Form.Item
          name="name"
          label="Nama grup"
          rules={[
            { required: true, whitespace: true, message: "Isi nama grup" },
          ]}
        >
          <Input maxLength={GROUP_NAME_MAX} showCount />
        </Form.Item>
        <Form.Item
          name="participants"
          label="Nomor anggota"
          extra="Satu nomor per baris, atau pisahkan dengan koma."
          rules={[
            {
              validator: (_, value?: string) =>
                parseParticipants(value ?? "").length > 0
                  ? Promise.resolve()
                  : Promise.reject(new Error("Isi minimal satu nomor")),
            },
          ]}
        >
          <Input.TextArea autoSize={{ minRows: 3, maxRows: 8 }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
