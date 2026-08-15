import { Modal, Form, Input, Select, InputNumber, Button, Alert } from "antd";
import {
  type Olt,
  type CreateOltDto,
  type UpdateOltDto,
  OltProtocol,
} from "@/domain/entities";
import { useSites } from "@/application/hooks";
import { useEffect, useState } from "react";
import { CheckCircleOutlined, CloseCircleOutlined } from "@ant-design/icons";
import { OltRepository } from "@/infrastructure/repositories/OltRepository";

interface OltModalProps {
  open: boolean;
  olt?: Olt;
  onClose: () => void;
  onSubmit: (data: CreateOltDto | UpdateOltDto) => void;
  loading: boolean;
}

export function OltModal({
  open,
  olt,
  onClose,
  onSubmit,
  loading,
}: OltModalProps) {
  const [form] = Form.useForm();
  const { data: sites } = useSites();
  const [testLoading, setTestLoading] = useState(false);
  const [testResult, setTestResult] = useState<{
    success: boolean;
    passedTests: string[];
    failedTest?: string;
    failedReason?: string;
  } | null>(null);
  const oltRepository = new OltRepository();

  useEffect(() => {
    if (olt) {
      form.setFieldsValue({
        siteId: olt.siteId,
        name: olt.name,
        ipAddress: olt.ipAddress,
        preferredProtocol: olt.preferredProtocol,
        username: olt.username,
        snmpCommunity: olt.snmpCommunity,
        sshPort: olt.sshPort,
        telnetPort: olt.telnetPort,
        snmpPort: olt.snmpPort,
      });
    } else {
      form.resetFields();
      form.setFieldsValue({
        preferredProtocol: OltProtocol.SSH,
        sshPort: 22,
        telnetPort: 23,
        snmpPort: 161,
      });
    }
    setTestResult(null);
  }, [olt, form]);

  const handleTestConnection = async () => {
    try {
      const values = await form.validateFields([
        "ipAddress",
        "username",
        "password",
        "preferredProtocol",
        "sshPort",
        "telnetPort",
        "snmpPort",
        "snmpCommunity",
      ]);

      setTestLoading(true);
      setTestResult(null);

      const result = await oltRepository.testConnection({
        ipAddress: values.ipAddress,
        username: values.username,
        password: values.password,
        preferredProtocol: values.preferredProtocol,
        sshPort: values.sshPort || 22,
        telnetPort: values.telnetPort || 23,
        snmpPort: values.snmpPort || 161,
        snmpCommunity: values.snmpCommunity || "public",
      });

      setTestResult(result);
    } catch (error) {
      console.error("Test connection error:", error);
    } finally {
      setTestLoading(false);
    }
  };

  const handleSubmit = () => {
    form.validateFields().then((values) => {
      onSubmit(values);
    });
  };

  return (
    <Modal
      title={olt ? "Edit OLT" : "Create OLT"}
      open={open}
      onCancel={onClose}
      destroyOnClose
      width={600}
      footer={[
        <Button key="cancel" onClick={onClose}>
          Cancel
        </Button>,
        !olt && (
          <Button
            key="test"
            onClick={handleTestConnection}
            loading={testLoading}
          >
            Test Connection
          </Button>
        ),
        <Button
          key="submit"
          type="primary"
          onClick={handleSubmit}
          loading={loading}
          disabled={!olt && !!testResult && !testResult.success}
        >
          {olt ? "Update" : "OK"}
        </Button>,
      ]}
    >
      <Form form={form} layout="vertical">
        {testResult && (
          <Alert
            message={
              testResult.success
                ? "Connection Test Successful"
                : "Connection Test Failed"
            }
            description={
              testResult.success ? (
                <div>
                  <CheckCircleOutlined style={{ color: "#52c41a" }} /> Passed
                  tests: {testResult.passedTests.join(", ")}
                </div>
              ) : (
                <div>
                  <CloseCircleOutlined style={{ color: "#ff4d4f" }} /> Passed:{" "}
                  {testResult.passedTests.join(", ") || "None"} | Failed:{" "}
                  {testResult.failedTest} - {testResult.failedReason}
                </div>
              )
            }
            type={testResult.success ? "success" : "error"}
            showIcon
            style={{ marginBottom: 16 }}
          />
        )}
        <Form.Item
          name="siteId"
          label="Site"
          rules={[{ required: true, message: "Please select site" }]}
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
          rules={[{ required: true, message: "Please enter OLT name" }]}
        >
          <Input />
        </Form.Item>

        <Form.Item
          name="ipAddress"
          label="IP Address"
          rules={[
            { required: true, message: "Please enter IP address" },
            {
              pattern: /^(\d{1,3}\.){3}\d{1,3}$/,
              message: "Invalid IP address",
            },
          ]}
        >
          <Input placeholder="192.168.1.1" />
        </Form.Item>

        <Form.Item
          name="preferredProtocol"
          label="Protocol"
          rules={[{ required: true, message: "Please select protocol" }]}
        >
          <Select>
            <Select.Option value={OltProtocol.SSH}>SSH</Select.Option>
            <Select.Option value={OltProtocol.TELNET}>Telnet</Select.Option>
          </Select>
        </Form.Item>

        <Form.Item
          name="username"
          label="Username"
          rules={[{ required: true, message: "Please enter username" }]}
        >
          <Input />
        </Form.Item>

        {!olt && (
          <Form.Item
            name="password"
            label="Password"
            rules={[{ required: true, message: "Please enter password" }]}
          >
            <Input.Password />
          </Form.Item>
        )}

        <Form.Item name="snmpCommunity" label="SNMP Community">
          <Input placeholder="public" />
        </Form.Item>

        <Form.Item name="sshPort" label="SSH Port">
          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
        </Form.Item>

        <Form.Item name="telnetPort" label="Telnet Port">
          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
        </Form.Item>

        <Form.Item name="snmpPort" label="SNMP Port">
          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
