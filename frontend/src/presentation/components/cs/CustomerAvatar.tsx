import { Avatar } from "antd";
import { UserOutlined } from "@ant-design/icons";
import type { CsConversation } from "@/domain/entities";
import { API_ENDPOINTS } from "@/infrastructure/http/endpoints";
import { env } from "@/shared/config/env";

interface CustomerAvatarProps {
  conversation: CsConversation;
  size: number;
}

/**
 * A customer's WhatsApp profile photo, falling back to the person icon.
 *
 * Two fallbacks, not one. hasAvatar decides whether to ask at all — most
 * customers hide their photo from anyone outside their contacts, and pointing
 * every row at the endpoint would put a 404 per row on every refresh of the
 * inbox. antd's own fallback then covers the photo that was there when the
 * list loaded and is gone by the time the image is fetched.
 */
export function CustomerAvatar({ conversation, size }: CustomerAvatarProps) {
  const src = conversation.hasAvatar
    ? `${env.apiUrl}${API_ENDPOINTS.CS_AVATAR(conversation.id)}`
    : undefined;

  return <Avatar size={size} src={src} icon={<UserOutlined />} />;
}
