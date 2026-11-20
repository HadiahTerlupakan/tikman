import { useState } from "react";
import { Button, Card, Space, Table, Tag, Typography, message } from "antd";
import type { ColumnsType } from "antd/es/table";
import type { SwitchBackupSummary } from "@/domain/entities";
import {
  useSwitchBackup,
  useSwitchBackupDiff,
  useSwitchBackupNow,
  useSwitchBackups,
} from "@/application/hooks/useSwitches";
import { ConfigDiff } from "./ConfigDiff";

const MAX_PICKED = 2;
const REASON_TEXT: Record<string, string> = {
  daily: "Harian",
  manual: "Manual",
};

const columns: ColumnsType<SwitchBackupSummary> = [
  {
    title: "Waktu",
    render: (_, b) => new Date(b.takenAt).toLocaleString("id-ID"),
  },
  {
    title: "Asal",
    render: (_, b) => <Tag>{REASON_TEXT[b.reason] ?? b.reason}</Tag>,
  },
  { title: "Ukuran", render: (_, b) => `${b.size} B` },
];

function Viewer(props: { switchId: string; picked: SwitchBackupSummary[] }) {
  const { switchId, picked } = props;
  const [older, newer] = [...picked].sort((a, b) =>
    a.takenAt.localeCompare(b.takenAt),
  );
  const single = picked.length === 1 ? older.id : null;
  const { data: backup } = useSwitchBackup(switchId, single);
  const pair = picked.length === MAX_PICKED;
  const { data: diff } = useSwitchBackupDiff(
    switchId,
    pair ? older.id : null,
    pair ? newer.id : null,
  );
  if (pair) {
    return diff === undefined ? null : <ConfigDiff diff={diff} />;
  }
  if (!backup) {
    return (
      <Typography.Text type="secondary">
        Pilih satu backup untuk melihat isinya, atau dua untuk membandingkan.
      </Typography.Text>
    );
  }
  return (
    <pre style={{ margin: 0, overflowX: "auto", fontSize: 12 }}>
      {backup.masked}
    </pre>
  );
}

export function SwitchConfigTab(props: {
  switchId: string;
  canBackup: boolean;
}) {
  const { switchId, canBackup } = props;
  const { data: backups = [], isLoading } = useSwitchBackups(switchId);
  const backupNow = useSwitchBackupNow();
  const [pickedIds, setPickedIds] = useState<string[]>([]);
  const picked = backups.filter((b) => pickedIds.includes(b.id));

  const start = () =>
    backupNow.mutate(switchId, {
      onSuccess: () =>
        message.success("Backup dijadwalkan; hasilnya muncul sebentar lagi"),
      onError: () => message.error("Backup gagal dijadwalkan"),
    });

  return (
    <Space direction="vertical" size="middle" style={{ width: "100%" }}>
      <Card
        size="small"
        title="Backup config"
        extra={
          canBackup && (
            <Button loading={backupNow.isPending} onClick={start}>
              Backup sekarang
            </Button>
          )
        }
      >
        <Table
          size="small"
          rowKey="id"
          loading={isLoading}
          pagination={{ pageSize: 10, hideOnSinglePage: true }}
          columns={columns}
          dataSource={backups}
          rowSelection={{
            type: "checkbox",
            selectedRowKeys: pickedIds,
            onChange: (keys) =>
              setPickedIds(keys.map(String).slice(-MAX_PICKED)),
          }}
        />
      </Card>
      <Card size="small" title="Isi config (rahasia disamarkan)">
        <Viewer switchId={switchId} picked={picked} />
      </Card>
    </Space>
  );
}
