import { Modal, Form, Input } from 'antd';
import { type Site, type CreateSiteDto, type UpdateSiteDto } from '@/domain/entities';
import { useEffect } from 'react';

interface SiteModalProps {
  open: boolean;
  site?: Site;
  onClose: () => void;
  onSubmit: (data: CreateSiteDto | UpdateSiteDto) => void;
  loading: boolean;
}

export function SiteModal({ open, site, onClose, onSubmit, loading }: SiteModalProps) {
  const [form] = Form.useForm();

  useEffect(() => {
    if (site) {
      form.setFieldsValue({
        name: site.name,
        location: site.location,
        description: site.description,
      });
    } else {
      form.resetFields();
    }
  }, [site, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={site ? 'Edit Site' : 'Create Site'}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="name"
          label="Site Name"
          rules={[{ required: true, message: 'Please enter site name' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item name="location" label="Location">
          <Input />
        </Form.Item>

        <Form.Item name="description" label="Description">
          <Input.TextArea rows={4} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
