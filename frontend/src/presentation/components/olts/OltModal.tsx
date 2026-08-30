import {
  Modal,
  Form,
  Input,
  Select,
  InputNumber,
  Button,
  Alert,
  message,
} from "antd";
import {
  type Olt,
  type CreateOltDto,
  type UpdateOltDto,
  OltProtocol,
  OltModel,
  OLT_MODELS,
} from "@/domain/entities";
import { useSites } from "@/application/hooks";
import { LocationFields } from "@/presentation/components/common/LocationFields";
import { parseCoordinate } from "@/presentation/components/sites/siteCoordinates";
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
  const [selectedProtocol, setSelectedProtocol] = useState<OltProtocol>(
    OltProtocol.SSH,
  );
  const oltRepository = new OltRepository();

  useEffect(() => {
    if (!open) return; // Don't manipulate form before Modal opens

    if (olt) {
      form.setFieldsValue({
        siteId: olt.siteId,
        name: olt.name,
        ipAddress: olt.ipAddress,
        model: olt.model,
        preferredProtocol: olt.preferredProtocol,
        username: olt.username,
        snmpCommunity: olt.snmpCommunity,
        sshPort: olt.sshPort,
        telnetPort: olt.telnetPort,
        snmpPort: olt.snmpPort,
        latitude: olt.latitude?.toString() ?? "",
        longitude: olt.longitude?.toString() ?? "",
      });
      setSelectedProtocol(olt.preferredProtocol);
    } else {
      form.resetFields();
      form.setFieldsValue({
        preferredProtocol: OltProtocol.SSH,
        model: OltModel.ZTE_C300,
        sshPort: 22,
        telnetPort: 23,
        snmpPort: 161,
      });
      setSelectedProtocol(OltProtocol.SSH);
    }
    setTestResult(null);
  }, [olt, form, open]);

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
      // A rejection carrying errorFields is antd telling us the form is
      // incomplete; it already marks the offending inputs, so a toast would be
      // noise. Anything else is the request itself failing, and staying silent
      // there is what made a broken endpoint look like a dead button.
      if (error && typeof error === "object" && "errorFields" in error) {
        return;
      }
      message.error(
        error instanceof Error
          ? `Connection test failed: ${error.message}`
          : "Connection test failed",
      );
    } finally {
      setTestLoading(false);
    }
  };

  const handleSubmit = () => {
    form
      .validateFields()
      .then((values) => {
        const latitude = parseCoordinate(values.latitude ?? "");
        const longitude = parseCoordinate(values.longitude ?? "");
        const hadCoordinates =
          olt?.latitude !== undefined && olt?.longitude !== undefined;

        onSubmit({
          ...values,
          ...(latitude !== null && longitude !== null
            ? { latitude, longitude }
            : {}),
          // Only an edit can clear a pin, and only when the OLT had one: on a
          // new OLT there is nothing to remove.
          ...(hadCoordinates && latitude === null && longitude === null
            ? { clearCoordinates: true }
            : {}),
        });
      })
      // antd renders each failure against its own field, so there is nothing
      // left to report — but without this the rejection escapes unhandled.
      .catch(() => undefined);
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
          name="model"
          label="OLT Model"
          rules={[{ required: true, message: "Please select OLT model" }]}
        >
          <Select placeholder="Select model">
            {OLT_MODELS.map((m) => (
              <Select.Option key={m.value} value={m.value}>
                {m.label}
                {m.hint ? ` (${m.hint})` : ""}
              </Select.Option>
            ))}
          </Select>
        </Form.Item>

        <Form.Item
          name="preferredProtocol"
          label="Protocol"
          rules={[{ required: true, message: "Please select protocol" }]}
        >
          <Select
            onChange={(value) => setSelectedProtocol(value as OltProtocol)}
          >
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

        <LocationFields form={form} addressName="address" />

        <Form.Item name="snmpCommunity" label="SNMP Community">
          <Input placeholder="public" />
        </Form.Item>

        {selectedProtocol === OltProtocol.SSH && (
          <Form.Item name="sshPort" label="SSH Port">
            <InputNumber min={1} max={65535} style={{ width: "100%" }} />
          </Form.Item>
        )}

        {selectedProtocol === OltProtocol.TELNET && (
          <Form.Item name="telnetPort" label="Telnet Port">
            <InputNumber min={1} max={65535} style={{ width: "100%" }} />
          </Form.Item>
        )}

        <Form.Item name="snmpPort" label="SNMP Port">
          <InputNumber min={1} max={65535} style={{ width: "100%" }} />
        </Form.Item>
      </Form>
    </Modal>
  );
}
