import { useEffect, useState } from "react";
import { Input, Modal, Typography } from "antd";
import type { SettingStatus } from "@/domain/entities";

interface SettingValueModalProps {
  setting: SettingStatus | null;
  loading: boolean;
  onClose: () => void;
  onSubmit: (value: string) => void;
}

export function SettingValueModal({
  setting,
  loading,
  onClose,
  onSubmit,
}: SettingValueModalProps) {
  const [value, setValue] = useState("");

  useEffect(() => {
    // The stored value is never sent to this page, so the field always starts
    // empty rather than pretending to hold what is saved.
    setValue("");
  }, [setting]);

  return (
    <Modal
      open={!!setting}
      title={setting?.label}
      okText="Save"
      okButtonProps={{ disabled: !value.trim() }}
      confirmLoading={loading}
      onOk={() => onSubmit(value.trim())}
      onCancel={onClose}
      destroyOnClose
    >
      <Typography.Paragraph type="secondary">
        {setting?.description}
      </Typography.Paragraph>
      <Input.Password
        autoFocus
        value={value}
        placeholder="Paste the value"
        onChange={(event) => setValue(event.target.value)}
        onPressEnter={() => value.trim() && onSubmit(value.trim())}
      />
    </Modal>
  );
}
