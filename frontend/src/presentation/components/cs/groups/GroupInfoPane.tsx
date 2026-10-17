import { Button, Divider, Empty, Spin, Typography } from "antd";
import { ArrowLeftOutlined, HistoryOutlined } from "@ant-design/icons";
import type {
  WaGroup,
  WaGroupActionRequest,
  WaGroupParticipant,
} from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { GroupAvatar } from "./GroupAvatars";
import { GroupManagement } from "./GroupManagement";
import { ParticipantList } from "./ParticipantList";
import { StarredMessages } from "./StarredMessages";

const { Text, Paragraph } = Typography;

interface GroupInfoPaneProps {
  group?: WaGroup;
  participants: WaGroupParticipant[];
  loading: boolean;
  /** The viewer is a TikMan admin; management is theirs alone. */
  isAdmin: boolean;
  busy: boolean;
  onAction: (request: WaGroupActionRequest) => void;
  requestingHistory: boolean;
  onRequestHistory: () => void;
  /** Present only where the panes take turns. */
  onBack?: () => void;
}

const sectionTitle = {
  display: "block",
  fontSize: 11,
  textTransform: "uppercase",
  letterSpacing: 0.4,
  color: colors.textMuted,
  marginBottom: 6,
} as const;

/** Who is in the group and what it is about, with the admin's controls. */
export function GroupInfoPane({
  group,
  participants,
  loading,
  isAdmin,
  busy,
  onAction,
  requestingHistory,
  onRequestHistory,
  onBack,
}: GroupInfoPaneProps) {
  if (!group) {
    return (
      <Empty style={{ marginTop: 48 }} description="Belum ada grup dipilih" />
    );
  }

  // A group we left can no longer be changed by us, whoever is asking.
  const manages = isAdmin && !group.left;

  return (
    <div style={{ flex: 1, overflowY: "auto", padding: "12px 16px" }}>
      {onBack && (
        <Button
          type="text"
          icon={<ArrowLeftOutlined />}
          onClick={onBack}
          style={{ marginBottom: 8 }}
        >
          Kembali ke percakapan
        </Button>
      )}
      <div style={{ textAlign: "center", marginBottom: 8 }}>
        <GroupAvatar groupId={group.id} hasAvatar={group.hasAvatar} size={88} />
      </div>
      <Text strong style={{ display: "block", fontSize: 16 }}>
        {group.name || group.jid}
      </Text>
      <Paragraph
        type="secondary"
        style={{ fontSize: 13, whiteSpace: "pre-wrap", marginTop: 4 }}
        ellipsis={{ rows: 4, expandable: true, symbol: "selengkapnya" }}
      >
        {group.topic || "Tanpa deskripsi"}
      </Paragraph>

      {manages && (
        <>
          <Divider style={{ margin: "12px 0" }} />
          <Text style={sectionTitle}>Kelola grup</Text>
          <GroupManagement group={group} busy={busy} onAction={onAction} />
        </>
      )}

      <Divider style={{ margin: "12px 0" }} />
      <Text style={sectionTitle}>Pesan berbintang</Text>
      <StarredMessages groupId={group.id} />

      <Divider style={{ margin: "12px 0" }} />
      <Text style={sectionTitle}>{group.participantCount} anggota</Text>
      {loading ? (
        <Spin />
      ) : (
        <ParticipantList
          groupId={group.id}
          participants={participants}
          onAction={manages && group.isAdmin && !busy ? onAction : undefined}
        />
      )}

      <Divider style={{ margin: "12px 0" }} />
      <Button
        size="small"
        icon={<HistoryOutlined />}
        loading={requestingHistory}
        disabled={group.left}
        onClick={onRequestHistory}
      >
        Muat riwayat lama dari HP
      </Button>
      <Text
        type="secondary"
        style={{ display: "block", fontSize: 12, marginTop: 6 }}
      >
        Meminta HP mengirim sekitar 50 pesan sebelum pesan tertua yang
        tersimpan. HP harus sedang online, dan grup harus sudah punya setidaknya
        satu pesan di TikMan. Pesan muncul beberapa saat kemudian.
      </Text>
    </div>
  );
}
