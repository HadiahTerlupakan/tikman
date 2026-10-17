import { Button, Divider, Empty, Spin, Typography } from "antd";
import { ArrowLeftOutlined } from "@ant-design/icons";
import type {
  WaGroup,
  WaGroupActionRequest,
  WaGroupParticipant,
} from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { GroupManagement } from "./GroupManagement";
import { ParticipantList } from "./ParticipantList";

const { Text, Paragraph } = Typography;

interface GroupInfoPaneProps {
  group?: WaGroup;
  participants: WaGroupParticipant[];
  loading: boolean;
  /** The viewer is a TikMan admin; management is theirs alone. */
  isAdmin: boolean;
  busy: boolean;
  onAction: (request: WaGroupActionRequest) => void;
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
      <Text style={sectionTitle}>{group.participantCount} anggota</Text>
      {loading ? (
        <Spin />
      ) : (
        <ParticipantList
          participants={participants}
          onAction={manages && group.isAdmin && !busy ? onAction : undefined}
        />
      )}
    </div>
  );
}
