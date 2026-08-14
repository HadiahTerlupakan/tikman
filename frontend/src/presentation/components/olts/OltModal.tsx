import { Modal, Form, Input, Select, InputNumber } from 'antd';
import { type Olt, type CreateOltDto, type UpdateOltDto, OltProtocol } from '@/domain/entities';
import { useSites } from '@/application/hooks';
import { useEffect } from 'react';

interface OltModalProps {
  open: boolean;
  olt?: Olt;
  onClose: () => void;
  onSubmit: (data: CreateOltDto | UpdateOltDto) => void;
  loading: boolean;
}

export function OltModal({ open, olt, onClose, onSubmit, loading }: OltModalProps) {
  const [form] = Form.useForm();
  const { data: sites } = useSites();

  useEffect(() => {
    if (olt) {
      form.setFieldsValue({
        siteId: olt.siteId,
        name: olt.name,
        ipAddress: olt.ipAddress,
        protocol: olt.protocol,
        username: olt.username,
        snmpCommunity: olt.snmpCommunity,
        sshPort: olt.sshPort,
        telnetPort: olt.telnetPort,
        snmpPort: olt.snmpPort,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({
        protocol: OltProtocol.SSH,
        sshPort: 22,
        telnetPort: 23,
        snmpPort: 161,
      });
    }
  }, [olt, form]);

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={olt ? 'Edit OLT' : 'Create OLT'}
      open={open}
      onOk={handleSubmit}
      onCancel={onClose}
      confirmLoading={loading}
      destroyOnClose
      width={600}
    >
      <Form form={form} layout="vertical">
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: 'Please select site' }]}
        >
          <Select placeholder="Select site">
            {sites?.map((site) => (
              <Select.Option key={site.id} value={site.id}>
                {site.name}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>

        <Form.Item
          name="name"
          label="OLT Name"
          rules={[{ required: true, message: 'Please enter OLT name' }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="ipAddress"
          label="IP Address"
          rules={[
            { required: true, message: 'Please enter IP address' },
            { pattern: /^(\d{1,3}\.){3}\d{1,3}$/, message: 'Invalid IP address' },
          ]}
        >
          <Input placeholder="192.168.1.1" />
        </Form.Item>

        <Form.Item
          name="protocol"
          label="Protocol"
          rules={[{ required: true, message: 'Please select protocol' }]}
        >
          <Select>
            <Select.Option value={OltProtocol.SSH}>SSH</Select.Option>
            <Select.Option value={OltProtocol.TELNET}>Telnet</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item
          name="username"
          label="Username"
          rules={[{ required: true, message: 'Please enter username' }]}
        >
          <Input />
        </Form.Item>

        {!olt && (
          <Form.Item
            name="password"
            label="Password"
            rules={[{ required: true, message: 'Please enter password' }]}
          >
            <Input.Password />
          </Form.Item>
        )}

        <Form.Item name="snmpCommunity" label="SNMP Community">
          <Input placeholder="public" />
        </Form.Item>

        <Form.Item name="sshPort" label="SSH Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="telnetPort" label="Telnet Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>

        <Form.Item name="snmpPort" label="SNMP Port">
          <InputNumber min={1} max={65535} style={{ width: '100%' }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
