import { Button, Space } from "antd";
import { NotificationOutlined, ThunderboltOutlined } from "@ant-design/icons";
import type { WaAccount } from "@/domain/entities";
import type { WaStreamStatus } from "@/application/hooks/useCsStream";
import type { PushPermission } from "@/infrastructure/firebase/messaging";
import { PushOptInButton } from "./PushOptInButton";
import { WaConnectionBadge } from "./WaConnectionBadge";

interface InboxHeaderActionsProps {
  isAdmin: boolean;
  accounts?: WaAccount[];
  // WaStreamStatus is already the per-number map, not one number's status.
  stream: WaStreamStatus;
  pushPermission: PushPermission;
  pushRequesting: boolean;
  onEnablePush: () => void;
  onOpenQuickReplies: () => void;
  onOpenNumbers: () => void;
  onOpenBroadcast: () => void;
}

/** The inbox header's controls, lifted out of CsInboxPage so that page stays
 * under the file-size limit it had already outgrown. */
export function InboxHeaderActions({
  isAdmin,
  accounts,
  stream,
  pushPermission,
  pushRequesting,
  onEnablePush,
  onOpenQuickReplies,
  onOpenNumbers,
  onOpenBroadcast,
}: InboxHeaderActionsProps) {
  return (
    // wrap, so four buttons fall onto a second line on a narrow screen
    // rather than running off the side of it.
    <Space wrap>
      {isAdmin && (
        <Button icon={<ThunderboltOutlined />} onClick={onOpenQuickReplies}>
          Balasan Cepat
        </Button>
      )}
      {/* Deliberately not behind isAdmin, unlike the button beside it:
          broadcasting is open to every role that can open the inbox. */}
      <Button icon={<NotificationOutlined />} onClick={onOpenBroadcast}>
        Pengumuman
      </Button>
      <PushOptInButton
        permission={pushPermission}
        requesting={pushRequesting}
        onEnable={onEnablePush}
      />
      <WaConnectionBadge
        accounts={accounts}
        stream={stream}
        onOpenNumbers={isAdmin ? onOpenNumbers : undefined}
      />
    </Space>
  );
}
