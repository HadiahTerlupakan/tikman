import { useEffect, useMemo } from "react";
import { Button, Typography } from "antd";
import { CloseOutlined, FileOutlined } from "@ant-design/icons";
import { colors } from "@/shared/theme/colors";
import { formatBytes } from "../trafficFormat";

const { Text } = Typography;

const THUMBNAIL_PX = 44;

interface PendingAttachmentProps {
  file: File;
  onCancel: () => void;
}

/**
 * The attachment about to leave, shown above the box until the CS sends or
 * withdraws it — a screenshot pasted into the wrong thread is caught here
 * rather than on the customer's phone.
 */
export function PendingAttachment({ file, onCancel }: PendingAttachmentProps) {
  const thumbnail = useMemo(
    () =>
      file.type.startsWith("image/") ? URL.createObjectURL(file) : undefined,
    [file],
  );

  useEffect(
    () => () => {
      if (thumbnail) URL.revokeObjectURL(thumbnail);
    },
    [thumbnail],
  );

  return (
    <div
      style={{ display: "flex", alignItems: "center", gap: 8, marginBottom: 6 }}
    >
      {thumbnail ? (
        <img
          src={thumbnail}
          alt={file.name}
          style={{
            width: THUMBNAIL_PX,
            height: THUMBNAIL_PX,
            objectFit: "cover",
            borderRadius: 6,
          }}
        />
      ) : (
        <FileOutlined style={{ fontSize: 24, color: colors.textSecondary }} />
      )}
      <div style={{ flex: 1, minWidth: 0 }}>
        <Text ellipsis style={{ display: "block", fontSize: 13 }}>
          {file.name}
        </Text>
        <Text type="secondary" style={{ fontSize: 12 }}>
          {formatBytes(file.size)}
        </Text>
      </div>
      <Button
        type="text"
        size="small"
        icon={<CloseOutlined />}
        onClick={onCancel}
        aria-label="Batalkan lampiran"
        title="Batalkan lampiran"
      />
    </div>
  );
}
