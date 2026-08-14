import { Modal, Form, Input, Select } from 'antd';
import { UserRole, type User, type CreateUserDto, type UpdateUserDto } from '@/domain/entities';
import { useEffect } from 'react';

interface UserModalProps {
  open: boolean;
  user?: User;
  onClose: () => void;
  onSubmit: (data: CreateUserDto | UpdateUserDto) => void;
  loading: boolean;
}

export function UserModal({ open, user, onClose, onSubmit, loading }: UserModalProps) {
  const [form] = Form.useForm();

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
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={user ? 'Edit User' : 'Create User'}
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
          rules={[{ required: true, message: 'Username harus diisi' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="email"
          label="Email"
          rules={[
            { required: true, message: 'Email harus diisi' },
            { type: 'email', message: 'Email tidak valid' },
          ]}
        >
          <Input />
        </Form.Item>

        {!user && (
          <Form.Item
            name="password"
            label="Password"
            rules={[
              { required: true, message: 'Password harus diisi' },
              { min: 6, message: 'Password minimal 6 karakter' },
            ]}
          >
            <Input.Password />
          </Form.Item>
        )}

        <Form.Item
          name="role"
          label="Role"
          rules={[{ required: true, message: 'Role harus dipilih' }]}
        >
          <Select>
            <Select.Option value={UserRole.ADMIN}>Admin</Select.Option>
            <Select.Option value={UserRole.TECHNICIAN}>Technician</Select.Option>
            <Select.Option value={UserRole.VIEWER}>Viewer</Select.Option>
          </Select>
        </Form.Item>
      </Form>
    </Modal>
  );
}
