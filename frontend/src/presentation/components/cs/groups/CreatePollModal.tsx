import { Button, Form, Input, Modal, Radio } from "antd";
import { DeleteOutlined, PlusOutlined } from "@ant-design/icons";
import type { WaGroupPollRequest } from "@/domain/entities";

/** WhatsApp's own bounds on a poll's options. */
export const POLL_MIN_OPTIONS = 2;
export const POLL_MAX_OPTIONS = 12;

interface PollForm {
  question: string;
  options: string[];
  multiple: boolean;
}

/** Votes are cast by option name, so two options with one name could not be
 * told apart. */
async function distinctOptions(_: unknown, options: string[] = []) {
  const names = options
    .map((o) => (o ?? "").trim().toLowerCase())
    .filter(Boolean);
  if (new Set(names).size !== names.length) {
    throw new Error("Setiap pilihan harus berbeda");
  }
}

/** Writes a poll: a question, two to twelve options, single or multiple
 * choice. */
export function CreatePollModal({
  open,
  creating,
  onCreate,
  onClose,
}: {
  open: boolean;
  creating: boolean;
  onCreate: (poll: WaGroupPollRequest) => void;
  onClose: () => void;
}) {
  const [form] = Form.useForm<PollForm>();

  const submit = (values: PollForm) =>
    onCreate({
      question: values.question.trim(),
      options: values.options.map((o) => o.trim()),
      selectableCount: values.multiple ? 0 : 1,
    });

  return (
    <Modal
      open={open}
      title="Buat polling"
      okText="Kirim"
      cancelText="Batal"
      okButtonProps={{ loading: creating }}
      onOk={() => form.submit()}
      onCancel={onClose}
      destroyOnHidden
    >
      <Form
        form={form}
        layout="vertical"
        preserve={false}
        initialValues={{ question: "", options: ["", ""], multiple: false }}
        onFinish={submit}
      >
        <Form.Item
          name="question"
          label="Pertanyaan"
          rules={[
            { required: true, whitespace: true, message: "Isi pertanyaan" },
          ]}
        >
          <Input autoFocus maxLength={255} />
        </Form.Item>
        <Form.List name="options" rules={[{ validator: distinctOptions }]}>
          {(fields, { add, remove }, { errors }) => (
            <>
              {fields.map((field, i) => (
                <Form.Item
                  key={field.key}
                  label={i === 0 ? "Pilihan" : undefined}
                  style={{ marginBottom: 8 }}
                >
                  <div style={{ display: "flex", gap: 6 }}>
                    <Form.Item
                      name={field.name}
                      noStyle
                      rules={[
                        {
                          required: true,
                          whitespace: true,
                          message: "Isi pilihan",
                        },
                      ]}
                    >
                      <Input
                        placeholder={`Pilihan ${i + 1}`}
                        aria-label={`Pilihan ${i + 1}`}
                        maxLength={100}
                      />
                    </Form.Item>
                    {fields.length > POLL_MIN_OPTIONS && (
                      <Button
                        type="text"
                        icon={<DeleteOutlined />}
                        aria-label={`Hapus pilihan ${i + 1}`}
                        onClick={() => remove(field.name)}
                      />
                    )}
                  </div>
                </Form.Item>
              ))}
              <Form.ErrorList errors={errors} />
              {fields.length < POLL_MAX_OPTIONS && (
                <Button
                  type="dashed"
                  block
                  icon={<PlusOutlined />}
                  onClick={() => add("")}
                  style={{ marginBottom: 12 }}
                >
                  Tambah pilihan
                </Button>
              )}
            </>
          )}
        </Form.List>
        <Form.Item name="multiple" label="Jawaban">
          <Radio.Group>
            <Radio value={false}>Satu pilihan</Radio>
            <Radio value>Boleh lebih dari satu</Radio>
          </Radio.Group>
        </Form.Item>
      </Form>
    </Modal>
  );
}
