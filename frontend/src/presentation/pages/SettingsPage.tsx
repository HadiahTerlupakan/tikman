import { useState } from "react";
import { App, Skeleton } from "antd";
import {
  useDeleteSetting,
  useSaveSetting,
  useSettings,
} from "@/application/hooks";
import { GOOGLE_MAPS_API_KEY, type SettingStatus } from "@/domain/entities";
import { PageHeader, DarkCard } from "../components/common";
import {
  MapsKeyGuidance,
  SettingRow,
  SettingValueModal,
} from "../components/settings";

export default function SettingsPage() {
  const { message, modal } = App.useApp();
  const { data: settings, isLoading } = useSettings();
  const saveMutation = useSaveSetting();
  const deleteMutation = useDeleteSetting();
  const [editing, setEditing] = useState<SettingStatus | null>(null);

  const handleSave = (value: string) => {
    if (!editing) return;
    saveMutation.mutate(
      { name: editing.name, value },
      {
        onSuccess: () => {
          message.success(`${editing.label} saved`);
          setEditing(null);
        },
        onError: (error) => message.error(error.message),
      },
    );
  };

  const handleDelete = (setting: SettingStatus) => {
    modal.confirm({
      title: `Remove ${setting.label}?`,
      content: "Anything relying on it stops working until it is set again.",
      okText: "Remove",
      okButtonProps: { danger: true },
      onOk: () =>
        deleteMutation.mutateAsync(setting.name).then(
          () => message.success(`${setting.label} removed`),
          (error: Error) => message.error(error.message),
        ),
    });
  };

  return (
    <div>
      <PageHeader
        title="Settings"
        description="Credentials for external integrations"
      />

      <DarkCard>
        {isLoading ? (
          <Skeleton active paragraph={{ rows: 3 }} title={false} />
        ) : (
          (settings ?? []).map((setting) => (
            <div key={setting.name}>
              <SettingRow
                setting={setting}
                onEdit={setEditing}
                onDelete={handleDelete}
              />
              {setting.name === GOOGLE_MAPS_API_KEY && <MapsKeyGuidance />}
            </div>
          ))
        )}
      </DarkCard>

      <SettingValueModal
        setting={editing}
        loading={saveMutation.isPending}
        onClose={() => setEditing(null)}
        onSubmit={handleSave}
      />
    </div>
  );
}
