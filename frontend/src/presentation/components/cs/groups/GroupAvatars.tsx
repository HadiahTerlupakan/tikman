import { Avatar } from "antd";
import { TeamOutlined, UserOutlined } from "@ant-design/icons";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";

/**
 * A group's photo, falling back to the group icon. hasAvatar decides whether
 * to ask at all, for the reason CustomerAvatar gives: a list that pointed
 * every row at the endpoint would put a 404 per photo-less group on every
 * refresh.
 */
export function GroupAvatar({
  groupId,
  hasAvatar,
  size,
}: {
  groupId: string;
  hasAvatar: boolean;
  size: number;
}) {
  const src = hasAvatar
    ? `${env.apiUrl}${API_ENDPOINTS.CS_GROUP_AVATAR(groupId)}`
    : undefined;
  return (
    <Avatar
      size={size}
      src={src}
      icon={<TeamOutlined />}
      style={{ flexShrink: 0 }}
    />
  );
}

/** One member's photo, served through the group they are seen in. */
export function MemberAvatar({
  groupId,
  jid,
  hasAvatar,
  size,
  color,
}: {
  groupId: string;
  jid: string;
  hasAvatar: boolean;
  size: number;
  /** The member's name colour, so a photo-less avatar still says who. */
  color?: string;
}) {
  const src = hasAvatar
    ? `${env.apiUrl}${API_ENDPOINTS.CS_GROUP_MEMBER_AVATAR(groupId, jid)}`
    : undefined;
  return (
    <Avatar
      size={size}
      src={src}
      icon={<UserOutlined />}
      style={{ flexShrink: 0, background: "#3f3f46", color }}
    />
  );
}
