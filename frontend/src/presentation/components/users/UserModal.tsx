import { Modal, Form, Input, Select } from "antd";
import {
  UserRole,
  type User,
  type CreateUserDto,
  type UpdateUserDto,
} from "@/domain/entities";
import { useEffect } from "react";

// The API rejects anything shorter, and a mismatch here is invisible to the
// operator: the request fails with a generic message and no field is marked.
const MIN_PASSWORD_LENGTH = 12;

interface UserModalProps {
  open: boolean;
  user?: User;
  onClose: () => void;
  onSubmit: (data: CreateUserDto | UpdateUserDto) => void;
  loading: boolean;
}

interface UserFormValues {
  username: string;
  email: string;
  role: UserRole;
  password?: string;
  passwordConfirm?: string;
}

export function UserModal({
  open,
  user,
  onClose,
  onSubmit,
  loading,
}: UserModalProps) {
  const [form] = Form.useForm<UserFormValues>();

  useEffect(() => {
    if (user) {
      form.setFieldsValue({
        username: user.username,
        email: user.email,
        role: user.role,
      });
    } else {
      form.resetFields();
    }
  }, [user, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      const { username, email, role, password } = values;

      // The confirmation never leaves the form, and an empty password means
      // "leave it alone" — sending "" would fail the API's minimum length and
      // read to the operator as a rejected edit.
      onSubmit(
        password
          ? { username, email, role, password }
          : { username, email, role },
      );
    });
  };

  return (
    <Modal
      title={user ? "Edit User" : "Create User"}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="username"
          label="Username"
          rules={[{ required: true, message: "Please enter username" }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="email"
          label="Email"
          rules={[
            { required: true, message: "Please enter email" },
            { type: "email", message: "Invalid email address" },
          ]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="password"
          label="Password"
          extra={user ? "Leave blank to keep the current password." : undefined}
          rules={[
            { required: !user, message: "Please enter password" },
            {
              min: MIN_PASSWORD_LENGTH,
              message: `Password must be at least ${MIN_PASSWORD_LENGTH} characters`,
            },
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>

        <Form.Item
          name="passwordConfirm"
          label="Confirm password"
          dependencies={["password"]}
          rules={[
            ({ getFieldValue }) => ({
              // Only demanded once a password is actually being set. A typo
              // here would lock the operator out of their own installation.
              validator(_rule, value) {
                const password = getFieldValue("password");
                if (!password || value === password) {
                  return Promise.resolve();
                }
                return Promise.reject(
                  new Error("The two passwords do not match"),
                );
              },
            }),
          ]}
        >
          <Input.Password autoComplete="new-password" />
        </Form.Item>

        <Form.Item
          name="role"
          label="Role"
          rules={[{ required: true, message: "Please select role" }]}
        >
          <Select>
            <Select.Option value={UserRole.ADMIN}>Admin</Select.Option>
            <Select.Option value={UserRole.TECHNICIAN}>
              Technician
            </Select.Option>
            <Select.Option value={UserRole.VIEWER}>Viewer</Select.Option>
          </Select>
        </Form.Item>
      </Form>
    </Modal>
  );
}
