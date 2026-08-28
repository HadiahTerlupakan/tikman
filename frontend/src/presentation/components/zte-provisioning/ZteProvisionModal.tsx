import { useEffect, useMemo, useState } from "react";
import { Alert, Form, Modal, Steps, Switch } from "antd";
import type {
  ZteGPONRegisterRequest,
  ZteProvisionTarget,
} from "@/domain/entities";
import { InternetServiceForm } from "./InternetServiceForm";
import { OnuIdentityForm } from "./OnuIdentityForm";
import { ZteCommandPreview } from "./ZteCommandPreview";

interface ZteProvisionModalProps {
  open: boolean;
  mode: "register" | "configure";
  target: ZteProvisionTarget;
  onClose: () => void;
  onSubmit: (data: ZteGPONRegisterRequest) => void;
  loading?: boolean;
  error?: Error | null;
}

export function ZteProvisionModal({
  open,
  mode,
  target,
  onClose,
  onSubmit,
  loading = false,
  error,
}: ZteProvisionModalProps) {
  const [form] = Form.useForm<ZteGPONRegisterRequest>();
  const [step, setStep] = useState(0);
  const [confirmed, setConfirmed] = useState(false);
  const watchedValues = Form.useWatch([], form);
  const values = useMemo(() => watchedValues || {}, [watchedValues]);
  const onuId =
    values.onuIdMode === "custom" ? values.onuId : target.onuId || 1;

  useEffect(() => {
    if (open) {
      form.setFieldsValue({
        oltId: target.oltId,
        card: target.card,
        pon: target.pon,
        onuIdMode: mode === "configure" ? "custom" : "auto",
        onuId: target.onuId || 0,
        serialNumber: target.serialNumber,
        onuType: target.onuType || "",
        useVeip: false,
        name: target.name || "",
        description: target.description || "",
        serviceEnabled: true,
        vlanMode: "tag",
        serviceType: "internet",
        wanMode: "wan_ip",
        wanIpMode: "pppoe",
      });
    }
  }, [form, mode, open, target]);

  const submit = async () => {
    try {
      const data = await form.validateFields();
      if (!confirmed) return;
      onSubmit({
        ...data,
        confirm: true,
        onuId: data.onuIdMode === "auto" ? 0 : data.onuId,
      });
    } catch {
      // Ant Design displays field-level validation errors.
    }
  };

  const title =
    mode === "register"
      ? "Register ZTE GPON ONU"
      : "Configure ZTE Internet Service";
  const previewRequest = useMemo(
    () => values as Partial<ZteGPONRegisterRequest>,
    [values],
  );

  return (
    <Modal
      title={title}
      open={open}
      onCancel={() => {
        setStep(0);
        setConfirmed(false);
        onClose();
      }}
      onOk={step === 2 ? submit : () => setStep((current) => current + 1)}
      okText={step === 2 ? "Submit" : "Next"}
      confirmLoading={loading}
      destroyOnClose
      width={720}
    >
      <Alert
        type="warning"
        showIcon
        message="Supported devices: ZTE C300 and C320. One Internet service only."
        style={{ marginBottom: 16 }}
      />
      {error && (
        <Alert
          type="error"
          showIcon
          message={error.message.replace(
            /password\s+\S+/gi,
            "password <redacted>",
          )}
          style={{ marginBottom: 16 }}
        />
      )}
      <Steps
        current={step}
        items={[
          { title: "ONU identity" },
          { title: "Internet service" },
          { title: "Review" },
        ]}
      />
      <Form form={form} layout="vertical" style={{ marginTop: 24 }}>
        {step === 0 && <OnuIdentityForm target={target} />}
        {step === 1 && <InternetServiceForm oltId={target.oltId} />}
        {step === 2 && (
          <ZteCommandPreview request={previewRequest} onuId={onuId} />
        )}
      </Form>
      {step === 2 && (
        <label style={{ display: "block", marginTop: 16 }}>
          <Switch
            checked={confirmed}
            onChange={setConfirmed}
            style={{ marginRight: 8 }}
          />
          I have reviewed this configuration and want to submit it
        </label>
      )}
    </Modal>
  );
}
