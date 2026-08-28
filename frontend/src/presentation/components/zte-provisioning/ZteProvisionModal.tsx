import { useEffect, useMemo, useRef, useState } from "react";
import { Alert, Form, Modal, Steps, Switch } from "antd";
import type {
  ZteGPONRegisterRequest,
  ZteProvisionTarget,
} from "@/domain/entities";
import { useOntServiceConfig } from "@/application/hooks/useOnts";
import { InternetServiceForm } from "./InternetServiceForm";
import { OnuIdentityForm } from "./OnuIdentityForm";
import { ZteCommandPreview } from "./ZteCommandPreview";

interface ZteProvisionModalProps {
  open: boolean;
  mode: "register" | "configure";
  target: ZteProvisionTarget;
  // Configuring an existing ONT opens on what it is already running, which is
  // read from this ONT's stored service config.
  ontId?: string;
  onClose: () => void;
  onSubmit: (data: ZteGPONRegisterRequest) => void;
  loading?: boolean;
  error?: Error | null;
}

export function ZteProvisionModal({
  open,
  mode,
  target,
  ontId,
  onClose,
  onSubmit,
  loading = false,
  error,
}: ZteProvisionModalProps) {
  const [form] = Form.useForm<ZteGPONRegisterRequest>();
  const prefilled = useRef(false);
  const { data: serviceConfig } = useOntServiceConfig(
    mode === "configure" ? ontId : undefined,
  );
  const [step, setStep] = useState(0);
  const [confirmed, setConfirmed] = useState(false);
  const watchedValues = Form.useWatch([], form);
  const values = useMemo(() => watchedValues || {}, [watchedValues]);

  useEffect(() => {
    if (!open) {
      prefilled.current = false;
      return;
    }
    // Applied once per opening: the stored service arrives a moment after the
    // modal does, and reapplying it would wipe out anything already typed.
    if (prefilled.current) return;
    if (mode === "configure" && ontId && serviceConfig === undefined) return;
    prefilled.current = true;

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
      vlanMode: serviceConfig?.vlanMode ?? "tag",
      serviceType: serviceConfig?.serviceType ?? "internet",
      vlanId: serviceConfig?.vlanId,
      downloadProfile: serviceConfig?.tcontProfile ?? "",
      uploadProfile: serviceConfig?.tcontProfile ?? "",
      wanMode: serviceConfig?.wanMode ?? "wan_ip",
      wanIpMode: serviceConfig?.wanIpMode ?? "pppoe",
      vlanProfile: serviceConfig?.vlanProfile ?? "",
      pppoeUsername: serviceConfig?.pppoeUsername ?? "",
      pppoePassword: serviceConfig?.pppoePassword ?? "",
    });
  }, [form, mode, ontId, open, serviceConfig, target]);

  // Ant Design validates and returns only the fields still mounted. Unmounting
  // a step as the wizard advances therefore emptied the payload: submitting
  // from the review step sent nothing but the confirmation, and the OLT
  // rejected it on the first field it checked. The steps stay mounted and are
  // hidden instead.
  const stepFields: string[][] = [
    ["card", "pon", "onuIdMode", "onuId", "serialNumber", "onuType"],
    [
      "vlanMode",
      "serviceType",
      "vlanId",
      "downloadProfile",
      "uploadProfile",
      "wanMode",
      "wanIpMode",
      "vlanProfile",
      "pppoeUsername",
      "pppoePassword",
    ],
  ];

  // Checked per step so a missing field is reported where it can be corrected,
  // not two screens later.
  const next = async () => {
    try {
      await form.validateFields(stepFields[step]);
      setStep((current) => current + 1);
    } catch {
      // Ant Design displays field-level validation errors.
    }
  };

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
    () => ({
      ...(values as ZteGPONRegisterRequest),
      oltId: target.oltId,
      onuId: values.onuIdMode === "auto" ? 0 : values.onuId,
      confirm: false,
    }),
    [target.oltId, values],
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
      onOk={step === 2 ? submit : next}
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
        <div style={{ display: step === 0 ? "block" : "none" }}>
          <OnuIdentityForm target={target} />
        </div>
        <div style={{ display: step === 1 ? "block" : "none" }}>
          <InternetServiceForm oltId={target.oltId} />
        </div>
        {step === 2 && (
          <ZteCommandPreview
            mode={mode}
            targetId={mode === "register" ? target.oltId : ontId}
            request={previewRequest}
          />
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
