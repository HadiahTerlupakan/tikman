import { useState } from "react";
import { Button, Form, Input, List, Modal, Popconfirm, Space } from "antd";
import { DeleteOutlined, EditOutlined } from "@ant-design/icons";
import {
  useCreateQuickReply,
  useDeleteQuickReply,
  useUpdateQuickReply,
} from "@/application/hooks";
import type { CsQuickReply } from "@/domain/entities";

interface QuickReplyManagerModalProps {
  open: boolean;
  onClose: () => void;
  quickReplies: CsQuickReply[];
}

interface QuickReplyFormValues {
  title: string;
  body: string;
}

/**
 * The templates the picker offers. Nothing in the app could create one, so
 * QuickReplyPicker could only ever say "Belum ada balasan cepat" — the CRUD
 * routes have been admin-only and unreachable since the module landed.
 */
export function QuickReplyManagerModal({
  open,
  onClose,
  quickReplies,
}: QuickReplyManagerModalProps) {
  const [form] = Form.useForm<QuickReplyFormValues>();
  const [editing, setEditing] = useState<CsQuickReply>();

  const create = useCreateQuickReply();
  const update = useUpdateQuickReply();
  const remove = useDeleteQuickReply();

  const startEdit = (reply: CsQuickReply) => {
    setEditing(reply);
    form.setFieldsValue({ title: reply.title, body: reply.body });
  };

  const stopEdit = () => {
    setEditing(undefined);
    form.resetFields();
  };

  const handleSubmit = (values: QuickReplyFormValues) => {
    const done = { onSuccess: stopEdit };
    if (editing) {
      update.mutate({ id: editing.id, ...values }, done);
      return;
    }
    create.mutate(values, done);
  };

  return (
    <Modal
      title="Balasan Cepat"
      open={open}
      onCancel={onClose}
      footer={null}
      width={640}
    >
      <List
        dataSource={quickReplies}
        locale={{ emptyText: "Belum ada balasan cepat" }}
        renderItem={(reply) => (
          <List.Item
            actions={[
              <Button
                key="edit"
                size="small"
                icon={<EditOutlined />}
                onClick={() => startEdit(reply)}
              />,
              <Popconfirm
                key="delete"
                title="Hapus balasan ini?"
                okText="Hapus"
                cancelText="Batal"
                onConfirm={() => remove.mutate(reply.id)}
              >
                <Button size="small" danger icon={<DeleteOutlined />} />
              </Popconfirm>,
            ]}
          >
            <List.Item.Meta title={reply.title} description={reply.body} />
          </List.Item>
        )}
      />

      <Form
        form={form}
        layout="vertical"
        onFinish={handleSubmit}
        style={{ marginTop: 16 }}
      >
        <Form.Item
          name="title"
          label="Judul"
          rules={[{ required: true, message: "Beri judul balasan ini" }]}
        >
          <Input placeholder="Gangguan massal" />
        </Form.Item>
        <Form.Item
          name="body"
          label="Isi"
          rules={[
            { required: true, message: "Isi balasan tidak boleh kosong" },
          ]}
        >
          <Input.TextArea
            rows={3}
            placeholder="Mohon maaf atas gangguannya..."
          />
        </Form.Item>
        <Space>
          <Button
            type="primary"
            htmlType="submit"
            loading={create.isPending || update.isPending}
          >
            {editing ? "Simpan" : "Tambah"}
          </Button>
          {editing && <Button onClick={stopEdit}>Batal</Button>}
        </Space>
      </Form>
    </Modal>
  );
}
