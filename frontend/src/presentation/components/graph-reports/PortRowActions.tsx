import { Button, Space } from "antd";
import {
  ArrowDownOutlined,
  ArrowUpOutlined,
  DeleteOutlined,
  EyeOutlined,
} from "@ant-design/icons";

interface PortRowActionsProps {
  index: number;
  count: number;
  canPreview: boolean;
  onMove: (from: number, to: number) => void;
  onRemove: (index: number) => void;
  onPreview: () => void;
}

/** Order here is the order the images arrive in the group. */
export function PortRowActions({
  index,
  count,
  canPreview,
  onMove,
  onRemove,
  onPreview,
}: PortRowActionsProps) {
  return (
    <Space style={{ marginBottom: 8 }}>
      <Button icon={<EyeOutlined />} disabled={!canPreview} onClick={onPreview}>
        Pratinjau
      </Button>
      <Button
        icon={<ArrowUpOutlined />}
        aria-label="Naikkan"
        disabled={index === 0}
        onClick={() => onMove(index, index - 1)}
      />
      <Button
        icon={<ArrowDownOutlined />}
        aria-label="Turunkan"
        disabled={index === count - 1}
        onClick={() => onMove(index, index + 1)}
      />
      <Button
        danger
        icon={<DeleteOutlined />}
        aria-label="Hapus port"
        disabled={count === 1}
        onClick={() => onRemove(index)}
      />
    </Space>
  );
}
