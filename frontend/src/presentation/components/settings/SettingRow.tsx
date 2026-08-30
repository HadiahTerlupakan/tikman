import { Button, Space, Typography } from "antd";
import type { SettingStatus } from "@/domain/entities";
import { colors } from "@/shared/theme";

interface SettingRowProps {
  setting: SettingStatus;
  onEdit: (setting: SettingStatus) => void;
  onDelete: (setting: SettingStatus) => void;
}

export function SettingRow({ setting, onEdit, onDelete }: SettingRowProps) {
  return (
    <div
      style={{
        display: "flex",
        justifyContent: "space-between",
        alignItems: "flex-start",
        gap: 16,
        padding: "14px 0",
        borderTop: `1px solid ${colors.border}`,
      }}
    >
      <div style={{ minWidth: 0 }}>
        <div style={{ color: colors.textPrimary, fontSize: 14 }}>
          {setting.label}
        </div>
        <div
          style={{ color: colors.textSecondary, fontSize: 12, marginTop: 4 }}
        >
          {setting.description}
        </div>
        <div style={{ marginTop: 8 }}>
          {setting.configured ? (
            <Typography.Text code>{setting.preview}</Typography.Text>
          ) : (
            <span style={{ color: colors.textMuted, fontSize: 12 }}>
              Not configured
            </span>
          )}
        </div>
      </div>

      <Space>
        <Button size="small" onClick={() => onEdit(setting)}>
          {setting.configured ? "Replace" : "Set value"}
        </Button>
        {/* Offered only when there is something to remove: a button that does
            nothing still invites a click. */}
        {setting.configured && (
          <Button size="small" danger onClick={() => onDelete(setting)}>
            Remove
          </Button>
        )}
      </Space>
    </div>
  );
}
