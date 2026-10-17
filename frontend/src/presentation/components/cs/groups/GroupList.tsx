import { useState } from "react";
import { Badge, Button, Dropdown, Tag, Typography } from "antd";
import { DownOutlined, PushpinFilled } from "@ant-design/icons";
import type { WaGroup } from "@/domain/entities";
import { colors } from "@/shared/theme/colors";
import { shortTime } from "../conversationSummary";
import { GroupAvatar } from "./GroupAvatars";
import { groupPreview, typingLine } from "./groupSummary";

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
  /** Who is writing in each group, by group id. */
  typing?: Record<string, string[]>;
  onSelect: (id: string) => void;
  /** Pins or unpins a group for the whole team. */
  onPin?: (group: WaGroup) => void;
}

/** The chevron WhatsApp shows on a hovered row, opening its menu. Clicks
 * stop here, or opening the menu would also open the group. */
function RowMenu({
  group,
  visible,
  onPin,
}: {
  group: WaGroup;
  visible: boolean;
  onPin: (group: WaGroup) => void;
}) {
  const [focused, setFocused] = useState(false);
  return (
    <span
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <Dropdown
        trigger={["click"]}
        menu={{
          items: [
            { key: "pin", label: group.pinned ? "Lepas sematan" : "Sematkan" },
          ],
          onClick: () => onPin(group),
        }}
      >
        <Button
          type="text"
          size="small"
          icon={<DownOutlined />}
          aria-label={`Menu grup ${group.name}`}
          onFocus={() => setFocused(true)}
          onBlur={() => setFocused(false)}
          style={{
            opacity: visible || focused ? 1 : 0,
            transition: "opacity 120ms",
          }}
        />
      </Dropdown>
    </span>
  );
}

function GroupRow({
  group,
  selected,
  accountLabel,
  typingNames,
  onSelect,
  onPin,
}: {
  group: WaGroup;
  selected: boolean;
  accountLabel?: string;
  typingNames: string[];
  onSelect: (id: string) => void;
  onPin?: (group: WaGroup) => void;
}) {
  const [hovered, setHovered] = useState(false);
  const unread = group.unreadCount > 0;
  const typing = typingLine(typingNames);
  const previewColor = unread ? colors.textBody : colors.textSecondary;
  return (
    <div
      role="listitem"
      tabIndex={0}
      aria-label={group.name}
      onMouseEnter={() => setHovered(true)}
      onMouseLeave={() => setHovered(false)}
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
      <GroupAvatar groupId={group.id} hasAvatar={group.hasAvatar} size={42} />
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
              color: typing ? colors.success : previewColor,
            }}
          >
            {typing ?? groupPreview(group)}
          </Text>
          {group.pinned && (
            <PushpinFilled
              aria-label="Disematkan"
              style={{ color: colors.textMuted, fontSize: 12 }}
            />
          )}
          {onPin && (
            <RowMenu
              group={group}
              visible={hovered || selected}
              onPin={onPin}
            />
          )}
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

/** The groups of every connected number: pinned ones first, then newest
 * activity, in the order the server sends them. */
export function GroupList({
  groups,
  selectedId,
  accountLabels,
  typing = {},
  onSelect,
  onPin,
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
          typingNames={typing[group.id] ?? []}
          onSelect={onSelect}
          onPin={onPin}
        />
      ))}
    </div>
  );
}
