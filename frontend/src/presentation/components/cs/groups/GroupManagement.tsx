import { useState } from "react";
import { Button, Popconfirm, Space, Switch, Typography } from "antd";
import {
  EditOutlined,
  LinkOutlined,
  LogoutOutlined,
  UserAddOutlined,
} from "@ant-design/icons";
import type { WaGroup, WaGroupActionRequest } from "@/domain/entities";
import { GroupFieldModal } from "./GroupFieldModal";
import { GROUP_NAME_MAX, mayEditInfo, parseParticipants } from "./groupSummary";

const { Text } = Typography;

type Field = "name" | "topic" | "add";

interface GroupManagementProps {
  group: WaGroup;
  busy: boolean;
  onAction: (request: WaGroupActionRequest) => void;
}

function Toggle({
  label,
  checked,
  disabled,
  loading,
  onChange,
}: {
  label: string;
  checked: boolean;
  disabled: boolean;
  loading: boolean;
  onChange: (enabled: boolean) => void;
}) {
  return (
    <label style={{ display: "flex", justifyContent: "space-between", gap: 8 }}>
      <Text style={{ fontSize: 13 }}>{label}</Text>
      <Switch
        size="small"
        checked={checked}
        disabled={disabled}
        loading={loading}
        onChange={onChange}
      />
    </label>
  );
}

function EditButtons({ group, busy, onAction }: GroupManagementProps) {
  const [field, setField] = useState<Field>();
  const editInfo = mayEditInfo(group);
  const close = () => setField(undefined);

  const submit = (value: string) => {
    if (field === "name") onAction({ action: "rename", name: value });
    if (field === "topic") onAction({ action: "set_topic", topic: value });
    if (field === "add") {
      onAction({ action: "add", participants: parseParticipants(value) });
    }
    close();
  };

  return (
    <Space wrap size={6}>
      <Button
        size="small"
        icon={<EditOutlined />}
        disabled={!editInfo || busy}
        onClick={() => setField("name")}
      >
        Ubah nama
      </Button>
      <Button
        size="small"
        icon={<EditOutlined />}
        disabled={!editInfo || busy}
        onClick={() => setField("topic")}
      >
        Ubah deskripsi
      </Button>
      <Button
        size="small"
        icon={<UserAddOutlined />}
        disabled={!group.isAdmin || busy}
        onClick={() => setField("add")}
      >
        Tambah anggota
      </Button>
      <GroupFieldModal
        open={field === "name"}
        title="Ubah nama grup"
        initialValue={group.name}
        maxLength={GROUP_NAME_MAX}
        okText="Simpan"
        onSubmit={submit}
        onClose={close}
      />
      <GroupFieldModal
        open={field === "topic"}
        title="Ubah deskripsi grup"
        initialValue={group.topic}
        multiline
        allowEmpty
        okText="Simpan"
        onSubmit={submit}
        onClose={close}
      />
      <GroupFieldModal
        open={field === "add"}
        title="Tambah anggota"
        hint="Satu nomor per baris, atau pisahkan dengan koma. 0812…, 62812…, dan +62812… sama-sama diterima."
        initialValue=""
        multiline
        okText="Tambahkan"
        onSubmit={submit}
        onClose={close}
      />
    </Space>
  );
}

function InviteLinkButtons({ group, busy, onAction }: GroupManagementProps) {
  const disabled = !group.isAdmin || busy;
  return (
    <Space wrap size={6}>
      <Button
        size="small"
        icon={<LinkOutlined />}
        disabled={disabled}
        onClick={() => onAction({ action: "invite_link" })}
      >
        Tautan undangan
      </Button>
      <Popconfirm
        title="Buat tautan undangan baru?"
        description="Tautan yang lama langsung tidak berlaku lagi."
        okText="Buat baru"
        okButtonProps={{ danger: true }}
        cancelText="Batal"
        disabled={disabled}
        onConfirm={() => onAction({ action: "reset_invite_link" })}
      >
        <Button size="small" disabled={disabled}>
          Reset tautan
        </Button>
      </Popconfirm>
    </Space>
  );
}

/**
 * What a TikMan admin can change about a group. Most of it also needs our
 * number to be a group admin, which WhatsApp enforces whoever asks; those
 * controls are disabled with the reason rather than offered and refused.
 */
export function GroupManagement(props: GroupManagementProps) {
  const { group, busy, onAction } = props;
  return (
    <Space direction="vertical" size={10} style={{ width: "100%" }}>
      {!group.isAdmin && (
        <Text type="secondary" style={{ fontSize: 12 }}>
          Nomor ini bukan admin grup, jadi anggota, pengaturan, dan tautan
          undangan tidak bisa diubah dari sini.
        </Text>
      )}
      <EditButtons {...props} />
      <Toggle
        label="Hanya admin yang boleh mengirim pesan"
        checked={group.announce}
        disabled={!group.isAdmin}
        loading={busy}
        onChange={(enabled) => onAction({ action: "set_announce", enabled })}
      />
      <Toggle
        label="Hanya admin yang boleh mengubah info grup"
        checked={group.locked}
        disabled={!group.isAdmin}
        loading={busy}
        onChange={(enabled) => onAction({ action: "set_locked", enabled })}
      />
      <InviteLinkButtons {...props} />
      <Popconfirm
        title="Keluarkan nomor ini dari grup?"
        description="Pesan grup tidak akan masuk lagi dan tidak bisa dikirim. Riwayat tetap tersimpan."
        okText="Keluar"
        okButtonProps={{ danger: true }}
        cancelText="Batal"
        disabled={busy}
        onConfirm={() => onAction({ action: "leave" })}
      >
        <Button size="small" danger icon={<LogoutOutlined />} disabled={busy}>
          Keluar dari grup
        </Button>
      </Popconfirm>
    </Space>
  );
}
