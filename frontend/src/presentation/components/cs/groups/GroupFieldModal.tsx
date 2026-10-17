import { useEffect, useState } from "react";
import { Input, Modal, Typography } from "antd";

const { Text } = Typography;

interface GroupFieldModalProps {
  open: boolean;
  title: string;
  /** Explains what goes in the box, under it. */
  hint?: string;
  initialValue: string;
  maxLength?: number;
  multiline?: boolean;
  /** An empty value is allowed only where it means something, such as
   * clearing a description. */
  allowEmpty?: boolean;
  okText: string;
  onSubmit: (value: string) => void;
  onClose: () => void;
}

/** One text field in a dialog: a group's name, its description, or the
 * numbers to add to it. */
export function GroupFieldModal({
  open,
  title,
  hint,
  initialValue,
  maxLength,
  multiline,
  allowEmpty,
  okText,
  onSubmit,
  onClose,
}: GroupFieldModalProps) {
  const [value, setValue] = useState(initialValue);

  useEffect(() => {
    if (open) setValue(initialValue);
  }, [open, initialValue]);

  const empty = !value.trim();
  const field = multiline ? (
    <Input.TextArea
      autoFocus
      value={value}
      maxLength={maxLength}
      showCount={!!maxLength}
      autoSize={{ minRows: 3, maxRows: 8 }}
      onChange={(e) => setValue(e.target.value)}
    />
  ) : (
    <Input
      autoFocus
      value={value}
      maxLength={maxLength}
      showCount={!!maxLength}
      onChange={(e) => setValue(e.target.value)}
      onPressEnter={() => !empty && onSubmit(value.trim())}
    />
  );

  return (
    <Modal
      open={open}
      title={title}
      okText={okText}
      cancelText="Batal"
      okButtonProps={{ disabled: empty && !allowEmpty }}
      onOk={() => onSubmit(value.trim())}
      onCancel={onClose}
      destroyOnHidden
    >
      {field}
      {hint && (
        <Text
          type="secondary"
          style={{ display: "block", fontSize: 12, marginTop: 6 }}
        >
          {hint}
        </Text>
      )}
    </Modal>
  );
}
