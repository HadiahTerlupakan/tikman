import { Avatar, Badge, Tag, Typography } from "antd";
import { TeamOutlined } from "@ant-design/icons";
import type { WaGroup } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { shortTime } from "../conversationSummary";
import { groupPreview } from "./groupSummary";

const { Text } = Typography;

const ellipsis = {
  overflow: "hidden",
  textOverflow: "ellipsis",
  whiteSpace: "nowrap",
} as const;

interface GroupListProps {
  groups: WaGroup[];
  selectedId?: string;
  /** Number labels by account id. Passed only when there is more than one
   * number, so a single-number install is not given a tag that says nothing. */
  accountLabels?: Record<string, string>;
  onSelect: (id: string) => void;
}

function GroupRow({
  group,
  selected,
  accountLabel,
  onSelect,
}: {
  group: WaGroup;
  selected: boolean;
  accountLabel?: string;
  onSelect: (id: string) => void;
}) {
  const unread = group.unreadCount > 0;
  return (
    <div
      role="listitem"
      tabIndex={0}
      aria-label={group.name}
      onClick={() => onSelect(group.id)}
      onKeyDown={(e) => {
        if (e.key === "Enter" || e.key === " ") {
          e.preventDefault();
          onSelect(group.id);
        }
      }}
      style={{
        display: "flex",
        gap: 12,
        alignItems: "center",
        padding: "10px 14px",
        cursor: "pointer",
        borderBottom: `1px solid ${colors.border}`,
        background: selected ? "rgba(62, 207, 142, 0.08)" : undefined,
        borderLeft: `2px solid ${selected ? colors.success : "transparent"}`,
        opacity: group.left ? 0.6 : 1,
      }}
    >
      <Avatar size={42} icon={<TeamOutlined />} style={{ flexShrink: 0 }} />
      <div style={{ flex: 1, minWidth: 0 }}>
        <div
          style={{ display: "flex", justifyContent: "space-between", gap: 8 }}
        >
          <Text strong={unread} style={{ ...ellipsis, fontSize: 14 }}>
            {group.name || group.jid}
          </Text>
          {group.lastMessageAt && (
            <Text
              style={{
                fontSize: 11,
                whiteSpace: "nowrap",
                color: unread ? colors.success : colors.textMuted,
              }}
            >
              {shortTime(group.lastMessageAt)}
            </Text>
          )}
        </div>
        <div style={{ display: "flex", gap: 8, alignItems: "center" }}>
          <Text
            style={{
              ...ellipsis,
              flex: 1,
              fontSize: 13,
              color: unread ? colors.textBody : colors.textSecondary,
            }}
          >
            {groupPreview(group)}
          </Text>
          <Badge
            count={group.unreadCount}
            style={{ backgroundColor: colors.success, color: "#0b1f16" }}
          />
        </div>
        {(group.left || accountLabel) && (
          <div style={{ display: "flex", gap: 4, marginTop: 2 }}>
            {group.left && (
              <Tag color="default" bordered={false} style={tagStyle}>
                Keluar
              </Tag>
            )}
            {accountLabel && (
              <Tag bordered={false} style={tagStyle}>
                {accountLabel}
              </Tag>
            )}
          </div>
        )}
      </div>
    </div>
  );
}

const tagStyle = {
  fontSize: 10,
  lineHeight: "16px",
  margin: 0,
  padding: "0 5px",
} as const;

/** The groups of every connected number, newest activity first. */
export function GroupList({
  groups,
  selectedId,
  accountLabels,
  onSelect,
}: GroupListProps) {
  if (groups.length === 0) {
    return (
      <div style={{ padding: 24, textAlign: "center" }}>
        <Text type="secondary" style={{ fontSize: 13 }}>
          Belum ada grup. Tekan muat ulang untuk membaca daftar grup dari
          WhatsApp.
        </Text>
      </div>
    );
  }

  return (
    <div role="list">
      {groups.map((group) => (
        <GroupRow
          key={group.id}
          group={group}
          selected={group.id === selectedId}
          accountLabel={accountLabels?.[group.waAccountId]}
          onSelect={onSelect}
        />
      ))}
    </div>
  );
}
