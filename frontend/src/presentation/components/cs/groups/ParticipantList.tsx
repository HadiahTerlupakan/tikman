import { Button, Popconfirm, Tag, Typography } from "antd";
import {
  ArrowDownOutlined,
  CrownOutlined,
  UserDeleteOutlined,
} from "@ant-design/icons";
import type {
  WaGroupActionRequest,
  WaGroupParticipant,
  WaGroupRole,
} from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { participantLabel } from "./groupSummary";

const { Text } = Typography;

const roleTags: Partial<Record<WaGroupRole, { label: string; color: string }>> =
  {
    superadmin: { label: "Pemilik", color: "gold" },
    admin: { label: "Admin", color: "green" },
  };

interface ParticipantListProps {
  participants: WaGroupParticipant[];
  /** Present only when the viewer is a TikMan admin and our number is an
   * admin of the group — the two things WhatsApp and the API both demand. */
  onAction?: (request: WaGroupActionRequest) => void;
}

function MemberActions({
  participant,
  onAction,
}: {
  participant: WaGroupParticipant;
  onAction: (request: WaGroupActionRequest) => void;
}) {
  // The owner cannot be demoted or removed by anyone else in WhatsApp.
  if (participant.isSelf || participant.role === "superadmin") return null;
  const name = participantLabel(participant);
  const target = [participant.jid];
  const promoted = participant.role === "admin";

  return (
    <>
      <Button
        type="text"
        size="small"
        icon={promoted ? <ArrowDownOutlined /> : <CrownOutlined />}
        title={promoted ? "Jadikan anggota biasa" : "Jadikan admin"}
        aria-label={`${promoted ? "Jadikan anggota biasa" : "Jadikan admin"}: ${name}`}
        onClick={() =>
          onAction({
            action: promoted ? "demote" : "promote",
            participants: target,
          })
        }
      />
      <Popconfirm
        title={`Keluarkan ${name} dari grup?`}
        description="Anggota ini harus diundang lagi untuk bisa kembali."
        okText="Keluarkan"
        okButtonProps={{ danger: true }}
        cancelText="Batal"
        onConfirm={() => onAction({ action: "remove", participants: target })}
      >
        <Button
          type="text"
          size="small"
          danger
          icon={<UserDeleteOutlined />}
          title="Keluarkan dari grup"
          aria-label={`Keluarkan ${name}`}
        />
      </Popconfirm>
    </>
  );
}

/** The group's members, admins first, each with their role. */
export function ParticipantList({
  participants,
  onAction,
}: ParticipantListProps) {
  const rank = (p: WaGroupParticipant) =>
    p.role === "superadmin" ? 0 : p.role === "admin" ? 1 : 2;
  const ordered = [...participants].sort((a, b) => rank(a) - rank(b));

  return (
    <div role="list" aria-label="Anggota grup">
      {ordered.map((participant) => {
        const tag = roleTags[participant.role];
        return (
          <div
            key={participant.jid}
            role="listitem"
            style={{
              display: "flex",
              alignItems: "center",
              gap: 6,
              padding: "6px 0",
              borderBottom: `1px solid ${colors.border}`,
            }}
          >
            <div style={{ flex: 1, minWidth: 0 }}>
              <Text ellipsis style={{ display: "block", fontSize: 13 }}>
                {participantLabel(participant)}
              </Text>
              {participant.name && participant.phone && (
                <Text type="secondary" style={{ fontSize: 11 }}>
                  +{participant.phone}
                </Text>
              )}
            </div>
            {tag && (
              <Tag color={tag.color} bordered={false} style={{ margin: 0 }}>
                {tag.label}
              </Tag>
            )}
            {onAction && (
              <MemberActions participant={participant} onAction={onAction} />
            )}
          </div>
        );
      })}
    </div>
  );
}
